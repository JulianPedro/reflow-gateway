package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MCPInstancePhase represents the current lifecycle phase of an MCPInstance.
type MCPInstancePhase string

const (
	MCPInstancePhasePending MCPInstancePhase = "Pending"
	MCPInstancePhaseRunning MCPInstancePhase = "Running"
	MCPInstancePhaseReady   MCPInstancePhase = "Ready"
	MCPInstancePhaseFailed  MCPInstancePhase = "Failed"
)

// MCPInstanceSpec defines the desired state of MCPInstance.
type MCPInstanceSpec struct {
	// Image is the container image for the MCP server.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Port is the port the MCP server listens on (default: 8080).
	// +kubebuilder:default=8080
	// +optional
	Port int32 `json:"port,omitempty"`

	// HealthPath is the HTTP path for readiness/liveness probes. Empty means no probe.
	// +optional
	HealthPath string `json:"healthPath,omitempty"`

	// SubjectKey is the isolation key (e.g., "user:<hash>").
	// +kubebuilder:validation:Required
	SubjectKey string `json:"subjectKey"`

	// TargetName is the name of the gateway target.
	// +kubebuilder:validation:Required
	TargetName string `json:"targetName"`

	// TargetID is the UUID of the gateway target.
	// +kubebuilder:validation:Required
	TargetID string `json:"targetID"`

	// Env contains additional environment variables for the container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// SecretRef is the name of the Secret containing environment variables (used via envFrom).
	// +optional
	SecretRef string `json:"secretRef,omitempty"`

	// Command overrides the container's entrypoint.
	// +optional
	Command []string `json:"command,omitempty"`

	// Args overrides the container's arguments.
	// +optional
	Args []string `json:"args,omitempty"`

	// Resources specifies compute resource requirements for the container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// IdleTTL is how long the instance can be idle before being garbage collected (e.g., "30m").
	// +kubebuilder:default="30m"
	// +optional
	IdleTTL string `json:"idleTTL,omitempty"`

	// MaxLifetime is the maximum duration the instance can run (e.g., "24h").
	// +kubebuilder:default="24h"
	// +optional
	MaxLifetime string `json:"maxLifetime,omitempty"`
}

// MCPInstanceStatus defines the observed state of MCPInstance.
type MCPInstanceStatus struct {
	// Phase is the current lifecycle phase.
	// +optional
	Phase MCPInstancePhase `json:"phase,omitempty"`

	// PodName is the name of the managed pod.
	// +optional
	PodName string `json:"podName,omitempty"`

	// ServiceURL is the in-cluster URL to reach the MCP server.
	// +optional
	ServiceURL string `json:"serviceURL,omitempty"`

	// StartedAt is when the instance was first created.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// LastUsedAt is updated by the gateway on each request (for idle GC).
	// +optional
	LastUsedAt *metav1.Time `json:"lastUsedAt,omitempty"`

	// Message provides additional human-readable status information.
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations of the instance's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetName`
// +kubebuilder:printcolumn:name="SubjectKey",type=string,JSONPath=`.spec.subjectKey`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// MCPInstance is the Schema for the mcpinstances API.
type MCPInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPInstanceSpec   `json:"spec,omitempty"`
	Status MCPInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPInstanceList contains a list of MCPInstance.
type MCPInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPInstance{}, &MCPInstanceList{})
}
