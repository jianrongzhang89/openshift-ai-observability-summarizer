/*
Copyright 2025.

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

package operators

import (
	"context"
	"fmt"
	"time"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// OperatorSpec defines an operator to be installed
type OperatorSpec struct {
	Name                string
	Namespace           string
	Channel             string
	CatalogSource       string
	CatalogNamespace    string
	SubscriptionName    string
	OperatorGroupName   string
	InstallPlanApproval olmv1alpha1.Approval
	// TargetNamespaces is the list of namespaces the operator should watch
	// If nil, the operator watches all namespaces (AllNamespaces mode)
	// If set to [op.Namespace], the operator watches its own namespace (OwnNamespace mode)
	TargetNamespaces []string
	// StartingCSV pins the subscription to a specific CSV version
	StartingCSV string
}

// RequiredOperators defines the list of operators to install
var RequiredOperators = []OperatorSpec{
	{
		Name:                "cluster-observability-operator",
		Namespace:           "openshift-cluster-observability-operator",
		Channel:             "stable",
		CatalogSource:       "redhat-operators",
		CatalogNamespace:    "openshift-marketplace",
		SubscriptionName:    "cluster-observability-operator",
		OperatorGroupName:   "cluster-observability-operator",
		InstallPlanApproval: olmv1alpha1.ApprovalAutomatic,
		TargetNamespaces:    nil, // AllNamespaces mode
	},
	{
		Name:                "opentelemetry-product",
		Namespace:           "openshift-opentelemetry-operator",
		Channel:             "stable",
		CatalogSource:       "redhat-operators",
		CatalogNamespace:    "openshift-marketplace",
		SubscriptionName:    "opentelemetry-product",
		OperatorGroupName:   "opentelemetry-operator",
		InstallPlanApproval: olmv1alpha1.ApprovalAutomatic,
		TargetNamespaces:    nil, // AllNamespaces mode
	},
	{
		Name:                "tempo-product",
		Namespace:           "openshift-tempo-operator",
		Channel:             "stable",
		CatalogSource:       "redhat-operators",
		CatalogNamespace:    "openshift-marketplace",
		SubscriptionName:    "tempo-product",
		OperatorGroupName:   "tempo-operator",
		InstallPlanApproval: olmv1alpha1.ApprovalManual,
		TargetNamespaces:    nil, // AllNamespaces mode
		StartingCSV:         "tempo-operator.v0.16.0-2",
	},
	{
		Name:                "cluster-logging",
		Namespace:           "openshift-logging",
		Channel:             "stable-6.3",
		CatalogSource:       "redhat-operators",
		CatalogNamespace:    "openshift-marketplace",
		SubscriptionName:    "cluster-logging",
		OperatorGroupName:   "logging-operator",
		InstallPlanApproval: olmv1alpha1.ApprovalAutomatic,
		TargetNamespaces:    []string{"openshift-logging"}, // OwnNamespace mode
	},
	{
		Name:                "loki-operator",
		Namespace:           "openshift-operators-redhat",
		Channel:             "stable-6.3",
		CatalogSource:       "redhat-operators",
		CatalogNamespace:    "openshift-marketplace",
		SubscriptionName:    "loki-operator",
		OperatorGroupName:   "loki-operator",
		InstallPlanApproval: olmv1alpha1.ApprovalAutomatic,
		TargetNamespaces:    nil, // AllNamespaces mode
	},
}

// Installer manages the installation of OpenShift operators
type Installer struct {
	client.Client
}

// applyNamespaceOverride applies namespace overrides to an operator spec
func applyNamespaceOverride(op OperatorSpec, overrides *NamespaceOverrides) OperatorSpec {
	switch op.Name {
	case "cluster-observability-operator":
		if overrides.ClusterObservability != "" {
			op.Namespace = overrides.ClusterObservability
		}
	case "opentelemetry-product":
		if overrides.OpenTelemetry != "" {
			op.Namespace = overrides.OpenTelemetry
		}
	case "tempo-product":
		if overrides.Tempo != "" {
			op.Namespace = overrides.Tempo
		}
	case "cluster-logging":
		if overrides.Logging != "" {
			op.Namespace = overrides.Logging
			// Update targetNamespaces for OwnNamespace mode
			if len(op.TargetNamespaces) > 0 {
				op.TargetNamespaces = []string{overrides.Logging}
			}
		}
	case "loki-operator":
		if overrides.Loki != "" {
			op.Namespace = overrides.Loki
		}
	}
	return op
}

// NewInstaller creates a new operator installer
func NewInstaller(c client.Client) *Installer {
	return &Installer{Client: c}
}

// NamespaceOverrides contains namespace overrides for operators
type NamespaceOverrides struct {
	ClusterObservability string
	OpenTelemetry        string
	Tempo                string
	Logging              string
	Loki                 string
}

// InstallAll installs all required operators
func (i *Installer) InstallAll(ctx context.Context) (map[string]string, error) {
	return i.InstallAllWithOverrides(ctx, nil)
}

// InstallAllWithOverrides installs all required operators with optional namespace overrides
func (i *Installer) InstallAllWithOverrides(ctx context.Context, overrides *NamespaceOverrides) (map[string]string, error) {
	logger := log.FromContext(ctx)
	installedVersions := make(map[string]string)

	for _, op := range RequiredOperators {
		// Apply namespace overrides if provided
		if overrides != nil {
			op = applyNamespaceOverride(op, overrides)
		}
		logger.Info("Installing operator", "name", op.Name, "namespace", op.Namespace)

		// Create namespace if it doesn't exist
		if err := i.ensureNamespace(ctx, op.Namespace); err != nil {
			return installedVersions, fmt.Errorf("failed to ensure namespace %s: %w", op.Namespace, err)
		}

		// Create OperatorGroup if it doesn't exist
		if err := i.ensureOperatorGroup(ctx, op); err != nil {
			return installedVersions, fmt.Errorf("failed to ensure OperatorGroup for %s: %w", op.Name, err)
		}

		// Create Subscription if it doesn't exist
		if err := i.ensureSubscription(ctx, op); err != nil {
			return installedVersions, fmt.Errorf("failed to ensure Subscription for %s: %w", op.Name, err)
		}

		// Approve InstallPlan if approval is manual
		if op.InstallPlanApproval == olmv1alpha1.ApprovalManual {
			if err := i.approveInstallPlan(ctx, op); err != nil {
				return installedVersions, fmt.Errorf("failed to approve InstallPlan for %s: %w", op.Name, err)
			}
		}

		// Wait for CSV to be ready
		csv, err := i.waitForCSV(ctx, op)
		if err != nil {
			return installedVersions, fmt.Errorf("failed to wait for CSV for %s: %w", op.Name, err)
		}

		installedVersions[op.Name] = csv.Spec.Version.String()
		logger.Info("Operator installed successfully", "name", op.Name, "version", csv.Spec.Version.String())
	}

	return installedVersions, nil
}

// ensureNamespace creates a namespace if it doesn't exist
func (i *Installer) ensureNamespace(ctx context.Context, name string) error {
	logger := log.FromContext(ctx)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	err := i.Get(ctx, types.NamespacedName{Name: name}, ns)
	if err == nil {
		logger.Info("Namespace already exists", "namespace", name)
		return nil
	}

	if !errors.IsNotFound(err) {
		return err
	}

	logger.Info("Creating namespace", "namespace", name)
	return i.Create(ctx, ns)
}

// ensureOperatorGroup creates an OperatorGroup if it doesn't exist
func (i *Installer) ensureOperatorGroup(ctx context.Context, op OperatorSpec) error {
	logger := log.FromContext(ctx)

	// List existing OperatorGroups in the namespace
	ogList := &olmv1.OperatorGroupList{}
	if err := i.List(ctx, ogList, client.InNamespace(op.Namespace)); err != nil {
		return err
	}

	// If an OperatorGroup already exists, use it
	if len(ogList.Items) > 0 {
		logger.Info("OperatorGroup already exists", "namespace", op.Namespace, "name", ogList.Items[0].Name)
		return nil
	}

	// Create new OperatorGroup
	og := &olmv1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      op.OperatorGroupName,
			Namespace: op.Namespace,
		},
		Spec: olmv1.OperatorGroupSpec{
			TargetNamespaces: op.TargetNamespaces,
		},
	}

	if len(op.TargetNamespaces) == 0 {
		logger.Info("Creating OperatorGroup (AllNamespaces mode)", "namespace", op.Namespace, "name", op.OperatorGroupName)
	} else {
		logger.Info("Creating OperatorGroup", "namespace", op.Namespace, "name", op.OperatorGroupName, "targetNamespaces", op.TargetNamespaces)
	}
	return i.Create(ctx, og)
}

// ensureSubscription creates a Subscription if it doesn't exist
func (i *Installer) ensureSubscription(ctx context.Context, op OperatorSpec) error {
	logger := log.FromContext(ctx)

	sub := &olmv1alpha1.Subscription{}
	err := i.Get(ctx, types.NamespacedName{Name: op.SubscriptionName, Namespace: op.Namespace}, sub)
	if err == nil {
		logger.Info("Subscription already exists", "name", op.SubscriptionName, "namespace", op.Namespace)
		return nil
	}

	if !errors.IsNotFound(err) {
		return err
	}

	// Create new Subscription
	subSpec := &olmv1alpha1.SubscriptionSpec{
		CatalogSource:          op.CatalogSource,
		CatalogSourceNamespace: op.CatalogNamespace,
		Channel:                op.Channel,
		Package:                op.Name,
		InstallPlanApproval:    op.InstallPlanApproval,
	}

	// Add StartingCSV if specified (pins to specific version)
	if op.StartingCSV != "" {
		subSpec.StartingCSV = op.StartingCSV
		logger.Info("Creating Subscription with pinned version", "name", op.SubscriptionName, "namespace", op.Namespace, "startingCSV", op.StartingCSV)
	} else {
		logger.Info("Creating Subscription", "name", op.SubscriptionName, "namespace", op.Namespace)
	}

	sub = &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      op.SubscriptionName,
			Namespace: op.Namespace,
		},
		Spec: subSpec,
	}

	return i.Create(ctx, sub)
}

// approveInstallPlan approves a pending InstallPlan for manual approval subscriptions
func (i *Installer) approveInstallPlan(ctx context.Context, op OperatorSpec) error {
	logger := log.FromContext(ctx)
	logger.Info("Waiting for InstallPlan to approve", "subscription", op.SubscriptionName)

	// Wait for InstallPlan to be created
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		sub := &olmv1alpha1.Subscription{}
		if err := i.Get(ctx, types.NamespacedName{Name: op.SubscriptionName, Namespace: op.Namespace}, sub); err != nil {
			return false, err
		}

		// Check if InstallPlan reference exists
		if sub.Status.InstallPlanRef == nil || sub.Status.InstallPlanRef.Name == "" {
			logger.Info("Waiting for InstallPlan to be created", "subscription", op.SubscriptionName)
			return false, nil
		}

		// Get the InstallPlan
		installPlan := &olmv1alpha1.InstallPlan{}
		if err := i.Get(ctx, types.NamespacedName{Name: sub.Status.InstallPlanRef.Name, Namespace: op.Namespace}, installPlan); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		// Approve the InstallPlan if not already approved
		if !installPlan.Spec.Approved {
			logger.Info("Approving InstallPlan", "name", installPlan.Name, "csv", op.StartingCSV)
			installPlan.Spec.Approved = true
			if err := i.Update(ctx, installPlan); err != nil {
				return false, err
			}
		}

		return true, nil
	})

	if err != nil {
		return fmt.Errorf("timeout waiting for InstallPlan: %w", err)
	}

	logger.Info("InstallPlan approved", "subscription", op.SubscriptionName)
	return nil
}

// waitForCSV waits for the ClusterServiceVersion to be ready
func (i *Installer) waitForCSV(ctx context.Context, op OperatorSpec) (*olmv1alpha1.ClusterServiceVersion, error) {
	logger := log.FromContext(ctx)
	logger.Info("Waiting for CSV to be ready", "subscription", op.SubscriptionName, "namespace", op.Namespace)

	var csv *olmv1alpha1.ClusterServiceVersion

	// Wait for Subscription to have a CSV installed
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		sub := &olmv1alpha1.Subscription{}
		if err := i.Get(ctx, types.NamespacedName{Name: op.SubscriptionName, Namespace: op.Namespace}, sub); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Subscription not found yet, retrying...", "name", op.SubscriptionName)
				return false, nil
			}
			return false, err
		}

		// Check if CSV is installed
		if sub.Status.InstalledCSV == "" {
			logger.Info("CSV not installed yet, retrying...", "subscription", op.SubscriptionName)
			return false, nil
		}

		// Get the CSV
		csv = &olmv1alpha1.ClusterServiceVersion{}
		if err := i.Get(ctx, types.NamespacedName{Name: sub.Status.InstalledCSV, Namespace: op.Namespace}, csv); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("CSV not found yet, retrying...", "csv", sub.Status.InstalledCSV)
				return false, nil
			}
			return false, err
		}

		// Check if CSV is in Succeeded phase
		if csv.Status.Phase != olmv1alpha1.CSVPhaseSucceeded {
			logger.Info("CSV not ready yet", "csv", csv.Name, "phase", csv.Status.Phase)
			return false, nil
		}

		logger.Info("CSV is ready", "csv", csv.Name, "phase", csv.Status.Phase)
		return true, nil
	})

	if err != nil {
		return nil, fmt.Errorf("timeout waiting for CSV: %w", err)
	}

	return csv, nil
}

// CheckOperatorStatus checks if an operator is installed and ready
func (i *Installer) CheckOperatorStatus(ctx context.Context, operatorName, namespace string) (bool, string, error) {
	logger := log.FromContext(ctx)

	// Find the subscription
	subList := &olmv1alpha1.SubscriptionList{}
	if err := i.List(ctx, subList, client.InNamespace(namespace)); err != nil {
		return false, "", err
	}

	for _, sub := range subList.Items {
		if sub.Spec.Package == operatorName {
			if sub.Status.InstalledCSV == "" {
				return false, "", nil
			}

			// Get the CSV
			csv := &olmv1alpha1.ClusterServiceVersion{}
			if err := i.Get(ctx, types.NamespacedName{Name: sub.Status.InstalledCSV, Namespace: namespace}, csv); err != nil {
				if errors.IsNotFound(err) {
					return false, "", nil
				}
				return false, "", err
			}

			if csv.Status.Phase == olmv1alpha1.CSVPhaseSucceeded {
				logger.Info("Operator is ready", "name", operatorName, "version", csv.Spec.Version.String())
				return true, csv.Spec.Version.String(), nil
			}

			logger.Info("Operator is not ready", "name", operatorName, "phase", csv.Status.Phase)
			return false, "", nil
		}
	}

	return false, "", nil
}

// GetAllOperatorStatus returns the status of all required operators
func (i *Installer) GetAllOperatorStatus(ctx context.Context) (map[string]string, error) {
	status := make(map[string]string)

	for _, op := range RequiredOperators {
		ready, version, err := i.CheckOperatorStatus(ctx, op.Name, op.Namespace)
		if err != nil {
			return status, err
		}

		if ready {
			status[op.Name] = version
		} else {
			status[op.Name] = "Not Ready"
		}
	}

	return status, nil
}
