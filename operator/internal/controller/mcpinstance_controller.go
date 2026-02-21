package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mcpv1alpha1 "github.com/reflow/gateway-operator/api/v1alpha1"
)

const (
	labelManagedBy  = "reflow.io/managed-by"
	labelTarget     = "reflow.io/target"
	labelSubjectKey = "reflow.io/subject-key"
	managerName     = "reflow-operator"
)

// MCPInstanceReconciler reconciles a MCPInstance object.
type MCPInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mcp.reflow.io,resources=mcpinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mcp.reflow.io,resources=mcpinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mcp.reflow.io,resources=mcpinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update;patch

func (r *MCPInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. Fetch the MCPInstance CR
	instance := &mcpv1alpha1.MCPInstance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// CR deleted, owner references will clean up children
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 2. Check GC: idle TTL and max lifetime
	if r.shouldGarbageCollect(instance) {
		logger.Info("Garbage collecting expired MCPInstance", "name", instance.Name)
		if err := r.Delete(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// 3. Verify Secret exists (if referenced)
	if instance.Spec.SecretRef != "" {
		secret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      instance.Spec.SecretRef,
			Namespace: instance.Namespace,
		}, secret); err != nil {
			if errors.IsNotFound(err) {
				return r.updateStatus(ctx, instance, mcpv1alpha1.MCPInstancePhaseFailed,
					fmt.Sprintf("Secret %q not found", instance.Spec.SecretRef))
			}
			return ctrl.Result{}, err
		}

		// Set owner reference on the Secret so it gets cleaned up with the CR
		if err := controllerutil.SetOwnerReference(instance, secret, r.Scheme); err == nil {
			if err := r.Update(ctx, secret); err != nil {
				logger.V(1).Info("Failed to set owner reference on secret", "error", err)
			}
		}
	}

	// 4. Ensure Pod
	pod, err := r.ensurePod(ctx, instance)
	if err != nil {
		return r.updateStatus(ctx, instance, mcpv1alpha1.MCPInstancePhaseFailed,
			fmt.Sprintf("Failed to ensure pod: %v", err))
	}

	// 5. Ensure Service
	if err := r.ensureService(ctx, instance); err != nil {
		return r.updateStatus(ctx, instance, mcpv1alpha1.MCPInstancePhaseFailed,
			fmt.Sprintf("Failed to ensure service: %v", err))
	}

	// 6. Update status based on pod state
	return r.syncStatus(ctx, instance, pod)
}

func (r *MCPInstanceReconciler) shouldGarbageCollect(instance *mcpv1alpha1.MCPInstance) bool {
	now := time.Now()

	// Check max lifetime
	if instance.Status.StartedAt != nil {
		maxLifetime, err := time.ParseDuration(instance.Spec.MaxLifetime)
		if err != nil {
			maxLifetime = 24 * time.Hour
		}
		if now.Sub(instance.Status.StartedAt.Time) > maxLifetime {
			return true
		}
	}

	// Check idle TTL
	if instance.Status.LastUsedAt != nil {
		idleTTL, err := time.ParseDuration(instance.Spec.IdleTTL)
		if err != nil {
			idleTTL = 30 * time.Minute
		}
		if now.Sub(instance.Status.LastUsedAt.Time) > idleTTL {
			return true
		}
	}

	return false
}

func (r *MCPInstanceReconciler) ensurePod(ctx context.Context, instance *mcpv1alpha1.MCPInstance) (*corev1.Pod, error) {
	podName := instance.Name
	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: instance.Namespace}, pod)
	if err == nil {
		return pod, nil // Pod already exists
	}
	if !errors.IsNotFound(err) {
		return nil, err
	}

	// Build pod spec
	port := instance.Spec.Port
	if port == 0 {
		port = 8080
	}
	labels := map[string]string{
		labelManagedBy:  managerName,
		labelTarget:     instance.Spec.TargetName,
		labelSubjectKey: sanitizeLabel(instance.Spec.SubjectKey),
	}

	container := corev1.Container{
		Name:  "mcp-server",
		Image: instance.Spec.Image,
		Ports: []corev1.ContainerPort{
			{
				Name:          "mcp",
				ContainerPort: port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: append(instance.Spec.Env, corev1.EnvVar{
			Name:  "MCP_PORT",
			Value: fmt.Sprintf("%d", port),
		}),
	}

	// Only add readiness probe if healthPath is configured
	if instance.Spec.HealthPath != "" {
		container.ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: instance.Spec.HealthPath,
					Port: intstr.FromInt32(port),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       5,
			TimeoutSeconds:      3,
			FailureThreshold:    6,
		}
	}

	if instance.Spec.Command != nil {
		container.Command = instance.Spec.Command
	}
	if instance.Spec.Args != nil {
		container.Args = instance.Spec.Args
	}
	if instance.Spec.Resources != nil {
		container.Resources = *instance.Spec.Resources
	}

	// Add envFrom for secret
	if instance.Spec.SecretRef != "" {
		container.EnvFrom = []corev1.EnvFromSource{
			{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Spec.SecretRef,
					},
				},
			},
		}
	}

	pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers:    []corev1.Container{container},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}

	// Set owner reference for cascading delete
	if err := controllerutil.SetControllerReference(instance, pod, r.Scheme); err != nil {
		return nil, err
	}

	if err := r.Create(ctx, pod); err != nil {
		return nil, err
	}

	return pod, nil
}

func (r *MCPInstanceReconciler) ensureService(ctx context.Context, instance *mcpv1alpha1.MCPInstance) error {
	svcName := instance.Name
	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: svcName, Namespace: instance.Namespace}, svc)
	if err == nil {
		return nil // Service already exists
	}
	if !errors.IsNotFound(err) {
		return err
	}

	port := instance.Spec.Port
	if port == 0 {
		port = 8080
	}

	svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: instance.Namespace,
			Labels: map[string]string{
				labelManagedBy: managerName,
				labelTarget:    instance.Spec.TargetName,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				labelManagedBy:  managerName,
				labelTarget:     instance.Spec.TargetName,
				labelSubjectKey: sanitizeLabel(instance.Spec.SubjectKey),
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "mcp",
					Port:       port,
					TargetPort: intstr.FromInt32(port),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(instance, svc, r.Scheme); err != nil {
		return err
	}

	return r.Create(ctx, svc)
}

func (r *MCPInstanceReconciler) syncStatus(ctx context.Context, instance *mcpv1alpha1.MCPInstance, pod *corev1.Pod) (ctrl.Result, error) {
	port := instance.Spec.Port
	if port == 0 {
		port = 8080
	}

	now := metav1.Now()

	switch pod.Status.Phase {
	case corev1.PodRunning:
		// Check if pod is ready
		ready := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}

		if ready {
			// Use ClusterIP instead of DNS name so the gateway can reach the service
			// even when running outside the cluster (e.g., in Docker)
			svc := &corev1.Service{}
			serviceURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
				instance.Name, instance.Namespace, port)
			if err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, svc); err == nil && svc.Spec.ClusterIP != "" {
				serviceURL = fmt.Sprintf("http://%s:%d", svc.Spec.ClusterIP, port)
			}

			instance.Status.Phase = mcpv1alpha1.MCPInstancePhaseReady
			instance.Status.PodName = pod.Name
			instance.Status.ServiceURL = serviceURL
			instance.Status.Message = "Pod is ready"
			if instance.Status.StartedAt == nil {
				instance.Status.StartedAt = &now
			}
			if instance.Status.LastUsedAt == nil {
				instance.Status.LastUsedAt = &now
			}
		} else {
			instance.Status.Phase = mcpv1alpha1.MCPInstancePhaseRunning
			instance.Status.PodName = pod.Name
			instance.Status.Message = "Pod running, waiting for readiness"
		}

	case corev1.PodFailed:
		instance.Status.Phase = mcpv1alpha1.MCPInstancePhaseFailed
		instance.Status.PodName = pod.Name
		instance.Status.Message = "Pod failed"

	default:
		instance.Status.Phase = mcpv1alpha1.MCPInstancePhasePending
		instance.Status.PodName = pod.Name
		instance.Status.Message = fmt.Sprintf("Pod phase: %s", pod.Status.Phase)
	}

	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue for periodic status sync and GC checks
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *MCPInstanceReconciler) updateStatus(ctx context.Context, instance *mcpv1alpha1.MCPInstance, phase mcpv1alpha1.MCPInstancePhase, message string) (ctrl.Result, error) {
	instance.Status.Phase = phase
	instance.Status.Message = message

	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if phase == mcpv1alpha1.MCPInstancePhaseFailed {
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MCPInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mcpv1alpha1.MCPInstance{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// sanitizeLabel ensures a value is valid for a Kubernetes label (max 63 chars, alphanumeric + hyphens).
func sanitizeLabel(s string) string {
	result := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			result = append(result, c)
		} else {
			result = append(result, '-')
		}
	}
	if len(result) > 63 {
		result = result[:63]
	}
	return string(result)
}
