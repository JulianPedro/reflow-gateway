package k8s

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/reflow/gateway/internal/database"
	"github.com/reflow/gateway/internal/mcp"
	"github.com/rs/zerolog/log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var mcpInstanceGVR = schema.GroupVersionResource{
	Group:    "mcp.reflow.io",
	Version:  "v1alpha1",
	Resource: "mcpinstances",
}

// ManagerConfig holds configuration for the Kubernetes manager.
type ManagerConfig struct {
	Namespace    string
	Kubeconfig   string // empty = in-cluster
	IdleTTL      time.Duration
	MaxLifetime  time.Duration
	GCInterval   time.Duration
	MaxInstances int
}

// Manager manages MCP server instances in Kubernetes via MCPInstance CRs.
type Manager struct {
	mu            sync.Mutex
	serviceURLs   map[string]string // subjectKey → cached service URL
	dynamicClient dynamic.Interface
	clientset     kubernetes.Interface
	namespace     string
	config        ManagerConfig
	stopGC        chan struct{}
}

// NewManager creates a new Kubernetes manager.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	var restConfig *rest.Config
	var err error

	if cfg.Kubeconfig != "" {
		restConfig, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	} else {
		restConfig, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	m := &Manager{
		serviceURLs:   make(map[string]string),
		dynamicClient: dynamicClient,
		clientset:     clientset,
		namespace:     cfg.Namespace,
		config:        cfg,
		stopGC:        make(chan struct{}),
	}

	go m.gcLoop()

	log.Info().
		Str("namespace", cfg.Namespace).
		Int("max_instances", cfg.MaxInstances).
		Msg("Kubernetes manager initialized")

	return m, nil
}

// GetOrCreate ensures a pod is running for the given subject key and returns a new MCP client.
// Each call returns a fresh client so each gateway session gets its own MCP session.
func (m *Manager) GetOrCreate(ctx context.Context, subjectKey string, target *database.Target, envConfigs map[string]string) (*mcp.Client, error) {
	m.mu.Lock()

	// Check cached service URL — pod already running
	if serviceURL, ok := m.serviceURLs[subjectKey]; ok {
		m.mu.Unlock()
		go m.touchLastUsed(context.Background(), buildCRName(target.Name, subjectKey))
		return mcp.NewClient(mcp.ClientConfig{
			URL:           serviceURL + "/mcp",
			TransportType: mcp.TransportStreamableHTTP,
		}), nil
	}

	m.mu.Unlock()

	crName := buildCRName(target.Name, subjectKey)

	// Check if CR already exists
	existing, err := m.dynamicClient.Resource(mcpInstanceGVR).Namespace(m.namespace).Get(ctx, crName, metav1.GetOptions{})
	if err == nil {
		// CR exists — check phase
		phase, _, _ := unstructured.NestedString(existing.Object, "status", "phase")
		serviceURL, _, _ := unstructured.NestedString(existing.Object, "status", "serviceURL")

		if phase == "Ready" && serviceURL != "" {
			m.mu.Lock()
			m.serviceURLs[subjectKey] = serviceURL
			m.mu.Unlock()

			go m.touchLastUsed(context.Background(), crName)
			return mcp.NewClient(mcp.ClientConfig{
				URL:           serviceURL + "/mcp",
				TransportType: mcp.TransportStreamableHTTP,
			}), nil
		}

		if phase == "Failed" {
			// Delete and recreate
			log.Warn().Str("cr", crName).Msg("MCPInstance in Failed state, recreating")
			_ = m.dynamicClient.Resource(mcpInstanceGVR).Namespace(m.namespace).Delete(ctx, crName, metav1.DeleteOptions{})
			// Wait briefly for deletion
			time.Sleep(2 * time.Second)
		} else {
			// Still pending/running — wait for ready
			return m.waitForReady(ctx, crName, subjectKey)
		}
	} else if !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to check MCPInstance: %w", err)
	}

	// Check capacity
	list, err := m.dynamicClient.Resource(mcpInstanceGVR).Namespace(m.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list MCPInstances: %w", err)
	}
	if len(list.Items) >= m.config.MaxInstances {
		return nil, fmt.Errorf("max Kubernetes instances reached (%d)", m.config.MaxInstances)
	}

	// Create Secret with env configs
	secretName := crName + "-env"
	if len(envConfigs) > 0 {
		secretData := make(map[string][]byte)
		for k, v := range envConfigs {
			secretData[k] = []byte(v)
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: m.namespace,
				Labels: map[string]string{
					"reflow.io/managed-by": "reflow-gateway",
					"reflow.io/target":     target.Name,
				},
			},
			Data: secretData,
		}

		_, err = m.clientset.CoreV1().Secrets(m.namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("failed to create secret: %w", err)
		}
	}

	// Create MCPInstance CR
	port := target.Port
	if port == 0 {
		port = 8080
	}

	cr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "mcp.reflow.io/v1alpha1",
			"kind":       "MCPInstance",
			"metadata": map[string]interface{}{
				"name":      crName,
				"namespace": m.namespace,
				"labels": map[string]interface{}{
					"reflow.io/managed-by": "reflow-gateway",
					"reflow.io/target":     target.Name,
				},
			},
			"spec": map[string]interface{}{
				"image":      target.Image,
				"port":       int64(port),
				"subjectKey": subjectKey,
				"targetName":  target.Name,
				"targetID":    target.ID.String(),
				"idleTTL":     m.config.IdleTTL.String(),
				"maxLifetime": m.config.MaxLifetime.String(),
			},
		},
	}

	// Add secretRef if we created a secret
	if len(envConfigs) > 0 {
		spec := cr.Object["spec"].(map[string]interface{})
		spec["secretRef"] = secretName
	}

	// Add healthPath if set (empty = no readiness probe)
	if target.HealthPath != "" {
		spec := cr.Object["spec"].(map[string]interface{})
		spec["healthPath"] = target.HealthPath
	}

	// Add command/args if set on target
	// Command overrides ENTRYPOINT, Args overrides CMD — they're independent in K8s
	if target.Command != "" || len(target.Args) > 0 {
		spec := cr.Object["spec"].(map[string]interface{})
		if target.Command != "" {
			spec["command"] = []interface{}{target.Command}
		}
		if len(target.Args) > 0 {
			args := make([]interface{}, len(target.Args))
			for i, a := range target.Args {
				args[i] = a
			}
			spec["args"] = args
		}
	}

	_, err = m.dynamicClient.Resource(mcpInstanceGVR).Namespace(m.namespace).Create(ctx, cr, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create MCPInstance: %w", err)
	}

	log.Info().
		Str("cr", crName).
		Str("target", target.Name).
		Str("subject_key", subjectKey).
		Str("image", target.Image).
		Msg("Created MCPInstance CR")

	return m.waitForReady(ctx, crName, subjectKey)
}

// waitForReady polls the MCPInstance CR until it reaches Ready phase.
func (m *Manager) waitForReady(ctx context.Context, crName string, subjectKey string) (*mcp.Client, error) {
	timeout := 2 * time.Minute
	pollInterval := 2 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		cr, err := m.dynamicClient.Resource(mcpInstanceGVR).Namespace(m.namespace).Get(ctx, crName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get MCPInstance: %w", err)
		}

		phase, _, _ := unstructured.NestedString(cr.Object, "status", "phase")
		serviceURL, _, _ := unstructured.NestedString(cr.Object, "status", "serviceURL")

		if phase == "Ready" && serviceURL != "" {
			// Wait for the service to actually accept connections
			// (pod may be Ready before the MCP server is listening)
			if err := m.waitForConnection(ctx, serviceURL, crName); err != nil {
				return nil, err
			}

			m.mu.Lock()
			m.serviceURLs[subjectKey] = serviceURL
			m.mu.Unlock()

			log.Info().
				Str("cr", crName).
				Str("service_url", serviceURL).
				Msg("MCPInstance is Ready")

			return mcp.NewClient(mcp.ClientConfig{
				URL:           serviceURL + "/mcp",
				TransportType: mcp.TransportStreamableHTTP,
			}), nil
		}

		if phase == "Failed" {
			message, _, _ := unstructured.NestedString(cr.Object, "status", "message")
			return nil, fmt.Errorf("MCPInstance %s failed: %s", crName, message)
		}

		time.Sleep(pollInterval)
	}

	return nil, fmt.Errorf("MCPInstance %s did not become ready within %s", crName, timeout)
}

// waitForConnection checks that the service is actually accepting TCP connections.
func (m *Manager) waitForConnection(ctx context.Context, serviceURL string, crName string) error {
	parsed, err := url.Parse(serviceURL)
	if err != nil {
		return fmt.Errorf("invalid service URL: %w", err)
	}

	addr := parsed.Host
	maxAttempts := 15
	interval := 2 * time.Second

	for i := 0; i < maxAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
			log.Debug().Str("cr", crName).Int("attempts", i+1).Msg("Service is accepting connections")
			return nil
		}

		log.Debug().Str("cr", crName).Int("attempt", i+1).Msg("Service not ready yet, retrying...")
		time.Sleep(interval)
	}

	return fmt.Errorf("service %s not reachable after %d attempts", addr, maxAttempts)
}

// touchLastUsed patches the MCPInstance status.lastUsedAt field.
func (m *Manager) touchLastUsed(ctx context.Context, crName string) {
	now := time.Now().UTC().Format(time.RFC3339)
	patch := []byte(fmt.Sprintf(`{"status":{"lastUsedAt":"%s"}}`, now))

	_, err := m.dynamicClient.Resource(mcpInstanceGVR).Namespace(m.namespace).Patch(
		ctx, crName, "application/merge-patch+json", patch, metav1.PatchOptions{}, "status",
	)
	if err != nil {
		log.Debug().Err(err).Str("cr", crName).Msg("Failed to touch lastUsedAt")
	}
}

// gcLoop periodically cleans cached clients for CRs that no longer exist.
func (m *Manager) gcLoop() {
	ticker := time.NewTicker(m.config.GCInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.gc()
		case <-m.stopGC:
			return
		}
	}
}

func (m *Manager) gc() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for subjectKey, serviceURL := range m.serviceURLs {
		// Check if the service is still reachable
		parsed, err := url.Parse(serviceURL)
		if err != nil {
			delete(m.serviceURLs, subjectKey)
			continue
		}
		conn, err := net.DialTimeout("tcp", parsed.Host, 2*time.Second)
		if err != nil {
			log.Debug().Str("subject_key", subjectKey).Msg("GC: removing stale service URL")
			delete(m.serviceURLs, subjectKey)
		} else {
			conn.Close()
		}
	}
}

// RestartTarget deletes all MCPInstances for a target and clears cached URLs.
// This forces pods to be recreated with updated config on next connection.
func (m *Manager) RestartTarget(ctx context.Context, targetName string) (int, error) {
	// List MCPInstances with target label
	list, err := m.dynamicClient.Resource(mcpInstanceGVR).Namespace(m.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "reflow.io/target=" + targetName,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list MCPInstances: %w", err)
	}

	deleted := 0
	for _, item := range list.Items {
		if err := m.dynamicClient.Resource(mcpInstanceGVR).Namespace(m.namespace).Delete(ctx, item.GetName(), metav1.DeleteOptions{}); err != nil {
			log.Warn().Err(err).Str("cr", item.GetName()).Msg("Failed to delete MCPInstance")
			continue
		}
		deleted++
	}

	// Clear cached service URLs for this target
	m.mu.Lock()
	for key := range m.serviceURLs {
		delete(m.serviceURLs, key)
	}
	m.mu.Unlock()

	if deleted > 0 {
		log.Info().Str("target", targetName).Int("deleted", deleted).Msg("Restarted MCPInstances for target")
	}

	return deleted, nil
}

// Shutdown clears cached service URLs.
func (m *Manager) Shutdown() {
	close(m.stopGC)

	m.mu.Lock()
	defer m.mu.Unlock()

	log.Info().Int("count", len(m.serviceURLs)).Msg("Shutting down Kubernetes manager")
	m.serviceURLs = make(map[string]string)
}

// buildCRName creates a Kubernetes-safe name from target name and subject key.
// Max 63 chars, lowercase alphanumeric + hyphens.
func buildCRName(targetName, subjectKey string) string {
	raw := targetName + "-" + subjectKey
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	name := reg.ReplaceAllString(strings.ToLower(raw), "-")
	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")
	// Collapse multiple hyphens
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	if len(name) > 63 {
		name = name[:63]
	}
	name = strings.TrimRight(name, "-")
	return name
}

