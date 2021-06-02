package deployments

import (
	"context"
	"fmt"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Linkerd struct {
	Debug   bool
	Timeout time.Duration
}

const (
	LinkerdDeploymentID     = "linkerd"
	linkerdVersion          = "2.10.2"
	linkerdRolesYAML        = "linkerd/rbac.yaml"
	linkerdInstallJobYAML   = "linkerd/install-job.yaml"
	linkerdUninstallJobYAML = "linkerd/uninstall-job.yaml"
)

func (k *Linkerd) ID() string {
	return LinkerdDeploymentID
}

func (k *Linkerd) Backup(c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k *Linkerd) Restore(c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k Linkerd) Describe() string {
	return emoji.Sprintf(":cloud:Linkerd version: %s\n", linkerdVersion)
}

// Delete removes linkerd from kubernetes cluster
func (k Linkerd) Delete(c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Linkerd...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(LinkerdDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", LinkerdDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Linkerd because namespace either doesn't exist or not owned by Epinio")
		return nil
	}

	// Remove linkerd with the uninstall job
	if out, err := helpers.KubectlApplyEmbeddedYaml(linkerdUninstallJobYAML); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", linkerdUninstallJobYAML, out))
	}

	// The uninstall job also deletes the namespace.
	err = c.WaitForNamespaceMissing(ui, LinkerdDeploymentID, k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed to delete namespace")
	}

	// Now delete the service account too.
	if out, err := helpers.KubectlDeleteEmbeddedYaml(linkerdRolesYAML, true); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", linkerdUninstallJobYAML, out))
	}

	ui.Success().Msg("Linkerd removed")

	return nil
}

func (k Linkerd) apply(c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	if err := c.CreateNamespace(LinkerdDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
	}

	if out, err := helpers.KubectlApplyEmbeddedYaml(linkerdRolesYAML); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", linkerdUninstallJobYAML, out))
	}

	if out, err := helpers.KubectlApplyEmbeddedYaml(linkerdInstallJobYAML); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", linkerdInstallJobYAML, out))
	}

	if err := c.WaitForJobCompleted(LinkerdDeploymentID, "linkerd-install", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Linkerd install job to complete")
	}

	ui.Success().Msg("Linkerd deployed")

	return nil
}

func (k Linkerd) GetVersion() string {
	return linkerdVersion
}

func (k Linkerd) Deploy(c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	skipLinkerd, err := options.GetBool("skip-linkerd", LinkerdDeploymentID)
	if err != nil {
		return errors.Wrap(err, "Couldn't get skip-linkerd option")
	}
	if skipLinkerd {
		ui.Exclamation().Msg("Skipping Linkerd deployment by user request")
		return nil
	}

	_, err = c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		LinkerdDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + LinkerdDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Linkerd...")

	return k.apply(c, ui, options, false)
}

func (k Linkerd) Upgrade(c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		LinkerdDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + LinkerdDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Linkerd...")

	return k.apply(c, ui, options, true)
}
