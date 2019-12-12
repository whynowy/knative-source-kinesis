/*
Copyright 2019

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KinesisSource is the Schema for the AWS Kinesis API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type KinesisSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KinesisSourceSpec   `json:"spec,omitempty"`
	Status KinesisSourceStatus `json:"status,omitempty"`
}

// Check that KinesisSource can be validated and can be defaulted.
var _ runtime.Object = (*KinesisSource)(nil)

// Check that KinesisSource implements the Conditions duck type.
var _ = duck.VerifyType(&KinesisSource{}, &duckv1alpha1.Conditions{})

// KinesisSourceSpec defines the desired state of the source.
type KinesisSourceSpec struct {
	// Steam name.
	StreamName string `json:"streamName"`

	// Region
	Region string `json:"region"`

	// AwsCredsSecret is the credential used to poll the Kinesis data
	AwsCredsSecret corev1.SecretKeySelector `json:"awsCredsSecret,omitempty"`

	// KIAMOptions is the KIAM config used to poll Kinesis data
	KIAMOptions KiamOptions `json:"kiamOptions,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to
	// use as the sink.  This is where events will be received.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`

	// ServiceAccoutName is the name of the ServiceAccount that will be used to
	// run the Receive Adapter Deployment.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// KiamOptions defines the spec for KIAM configuration
type KiamOptions struct {
	// AssignedIAMRole is the IAM role that KIAM assigns to Receive Adapter,
	// It could be the IAM role ARN, or role name (if the role is in the same AWS account as k8s cluster nodes).
	AssignedIAMRole string `json:"assignedIamRole"`

	// KCLIAMRoleARN is the IAM role ARN,
	// this role needs to be configured to trust the AssignedIAMRole of Receive Adapter.
	KCLIAMRoleARN string `json:"kclIamRoleArn"`
}

const (
	// KinesisSourceConditionReady has status True when the source is
	// ready to send events.
	KinesisSourceConditionReady = duckv1alpha1.ConditionReady

	// KinesisSourceConditionSinkProvided has status True when the
	// KinesisSource has been configured with a sink target.
	KinesisSourceConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// KinesisSourceConditionDeployed has status True when the
	// KinesisSource has had it's receive adapter deployment created.
	KinesisSourceConditionDeployed duckv1alpha1.ConditionType = "Deployed"
)

var condSet = duckv1alpha1.NewLivingConditionSet(
	KinesisSourceConditionReady,
	KinesisSourceConditionSinkProvided,
	KinesisSourceConditionDeployed)

// KinesisSourceStatus defines the observed state of the source.
type KinesisSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the KinesisSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *KinesisSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return condSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *KinesisSourceStatus) IsReady() bool {
	return condSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *KinesisSourceStatus) InitializeConditions() {
	condSet.Manage(s).InitializeConditions()
}

// MarkSink sets the condition that the source has a sink configured.
func (s *KinesisSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		condSet.Manage(s).MarkTrue(KinesisSourceConditionSinkProvided)
	} else {
		condSet.Manage(s).MarkUnknown(KinesisSourceConditionSinkProvided,
			"SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *KinesisSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	condSet.Manage(s).MarkFalse(KinesisSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkDeployed sets the condition that the source has been deployed.
func (s *KinesisSourceStatus) MarkDeployed() {
	condSet.Manage(s).MarkTrue(KinesisSourceConditionDeployed)
}

// MarkDeploying sets the condition that the source is deploying.
func (s *KinesisSourceStatus) MarkDeploying(reason, messageFormat string, messageA ...interface{}) {
	condSet.Manage(s).MarkUnknown(KinesisSourceConditionDeployed, reason, messageFormat, messageA...)
}

// MarkNotDeployed sets the condition that the source has not been deployed.
func (s *KinesisSourceStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	condSet.Manage(s).MarkFalse(KinesisSourceConditionDeployed, reason, messageFormat, messageA...)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KinesisSourceList contains a list of KinesisSource
type KinesisSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KinesisSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KinesisSource{}, &KinesisSourceList{})
}
