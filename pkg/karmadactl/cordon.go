package karmadactl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
)

var (
	cordonShort   = `Mark cluster as unschedulable`
	cordonLong    = `Mark cluster as unschedulable.`
	cordonExample = `
# Mark cluster "foo" as unschedulable.
%s cordon foo
`

	uncordonShort   = `Mark cluster as schedulable`
	uncordonLong    = `Mark cluster as schedulable.`
	uncordonExample = `
# Mark cluster "foo" as schedulable.
%s uncordon foo
`
)

const (
	desiredCordon = iota
	desiredUnCordon
)

// NewCmdCordon defines the `cordon` command that mark cluster as unschedulable.
func NewCmdCordon(cmdOut io.Writer, karmadaConfig KarmadaConfig, cmdStr string) *cobra.Command {
	opts := CommandCordonOption{}
	cmd := &cobra.Command{
		Use:          "cordon CLUSTER",
		Short:        cordonShort,
		Long:         cordonLong,
		Example:      fmt.Sprintf(cordonExample, cmdStr),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(args); err != nil {
				return err
			}
			if err := RunCordonOrUncordon(cmdOut, desiredCordon, karmadaConfig, opts); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	opts.GlobalCommandOptions.AddFlags(flags)

	return cmd
}

// NewCmdUncordon defines the `uncordon` command that mark cluster as schedulable.
func NewCmdUncordon(cmdOut io.Writer, karmadaConfig KarmadaConfig, cmdStr string) *cobra.Command {
	opts := CommandCordonOption{}
	cmd := &cobra.Command{
		Use:          "uncordon CLUSTER",
		Short:        uncordonShort,
		Long:         uncordonLong,
		Example:      fmt.Sprintf(uncordonExample, cmdStr),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(args); err != nil {
				return err
			}
			if err := RunCordonOrUncordon(cmdOut, desiredUnCordon, karmadaConfig, opts); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	opts.AddFlags(flags)

	return cmd
}

// CommandCordonOption holds all command options for cordon and uncordon
type CommandCordonOption struct {
	// global flags
	options.GlobalCommandOptions

	// ClusterName is the cluster's name that we are going to join with.
	ClusterName string
}

// Complete ensures that options are valid and marshals them if necessary.
func (o *CommandCordonOption) Complete(args []string) error {
	// Get cluster name from the command args.
	if len(args) == 0 {
		return errors.New("cluster name is required")
	}
	o.ClusterName = args[0]
	return nil
}

// CordonHelper wraps functionality to cordon/uncordon cluster
type CordonHelper struct {
	cluster *clusterv1alpha1.Cluster
	desired int
}

// NewCordonHelper returns a new CordonHelper that help execute
// the cordon and uncordon commands
func NewCordonHelper(cluster *clusterv1alpha1.Cluster) *CordonHelper {
	return &CordonHelper{
		cluster: cluster,
	}
}

// UpdateIfRequired returns true if unscheduler taint isn't already set,
// or false when no change is needed
func (c *CordonHelper) UpdateIfRequired(desired int) bool {
	c.desired = desired

	if desired == desiredCordon && !c.hasUnschedulerTaint() {
		return true
	}

	if desired == desiredUnCordon && c.hasUnschedulerTaint() {
		return true
	}

	return false
}

func (c *CordonHelper) hasUnschedulerTaint() bool {
	unschedulerTaint := corev1.Taint{
		Key:    clusterv1alpha1.TaintClusterUnscheduler,
		Effect: corev1.TaintEffectNoSchedule,
	}

	for _, taint := range c.cluster.Spec.Taints {
		if taint.MatchTaint(&unschedulerTaint) {
			return true
		}
	}

	return false
}

// PatchOrReplace uses given karmada clientset to update the cluster unschedulable scheduler, either by patching or
// updating the given cluster object; it may return error if the object cannot be encoded as
// JSON, or if either patch or update calls fail; it will also return error whenever creating a patch has failed
func (c *CordonHelper) PatchOrReplace(controlPlaneClient *karmadaclientset.Clientset) error {
	client := controlPlaneClient.ClusterV1alpha1().Clusters()
	oldData, err := json.Marshal(c.cluster)
	if err != nil {
		return err
	}

	unschedulerTaint := corev1.Taint{
		Key:    clusterv1alpha1.TaintClusterUnscheduler,
		Effect: corev1.TaintEffectNoSchedule,
	}

	if c.desired == desiredCordon {
		c.cluster.Spec.Taints = append(c.cluster.Spec.Taints, unschedulerTaint)
	}

	if c.desired == desiredUnCordon {
		for i, n := 0, len(c.cluster.Spec.Taints); i < n; i++ {
			if c.cluster.Spec.Taints[i].MatchTaint(&unschedulerTaint) {
				c.cluster.Spec.Taints[i] = c.cluster.Spec.Taints[n-1]
				c.cluster.Spec.Taints = c.cluster.Spec.Taints[:n-1]
				break
			}
		}
	}

	newData, err := json.Marshal(c.cluster)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, c.cluster)
	if err == nil {
		_, err = client.Patch(context.TODO(), c.cluster.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	} else {
		_, err = client.Update(context.TODO(), c.cluster, metav1.UpdateOptions{})
	}
	return err
}

// RunCordonOrUncordon exec marks the cluster unschedulable or schedulable according to desired.
// if true cordon cluster otherwise uncordon cluster.
func RunCordonOrUncordon(_ io.Writer, desired int, karmadaConfig KarmadaConfig, opts CommandCordonOption) error {
	cordonOrUncordon := "cordon"
	if desired == desiredUnCordon {
		cordonOrUncordon = "un" + cordonOrUncordon
	}

	// Get control plane kube-apiserver client
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(opts.KarmadaContext, opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			opts.KarmadaContext, opts.KubeConfig, err)
	}

	controlPlaneKarmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)

	cluster, err := controlPlaneKarmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), opts.ClusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	cordonHelper := NewCordonHelper(cluster)
	if !cordonHelper.UpdateIfRequired(desired) {
		fmt.Printf("%s cluster %s\n", cluster.Name, alreadyStr(desired))
		return nil
	}

	if !opts.DryRun {
		err := cordonHelper.PatchOrReplace(controlPlaneKarmadaClient)
		if err != nil {
			return err
		}
	}

	fmt.Printf("%s cluster %sed\n", cluster.Name, cordonOrUncordon)
	return nil
}

func alreadyStr(desired int) string {
	if desired == desiredCordon {
		return "already cordoned"
	}
	return "already uncordoned"
}
