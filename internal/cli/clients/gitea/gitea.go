// Package gitea deals with using gitea as a store for pushed applications and their deployment info
package gitea

import (
	"fmt"
	"strings"

	giteaSDK "code.gitea.io/sdk/gitea"
	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/cli/config"
	"github.com/pkg/errors"
)

// Client provides functionality for talking to a
// Gitea installation on Kubernetes
type Client struct {
	Client   *giteaSDK.Client
	Domain   string
	URL      string
	Username string
	Password string
}

const (
	GiteaCredentialsSecret = "gitea-creds"
)

var clientMemo *Client

// New loads the config and returns a new gitea client
func New() (*Client, error) {
	if clientMemo != nil {
		return clientMemo, nil
	}

	configConfig, err := config.Load()
	if err != nil {
		return nil, err
	}

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return nil, err
	}

	domain, err := getMainDomain(cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine epinio domain")
	}

	giteaURL, err := getGiteaURL(configConfig, cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve gitea host")
	}

	username, password, err := getGiteaCredentials(cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve gitea credentials")
	}

	client, err := giteaSDK.NewClient(giteaURL)
	if err != nil {
		return nil, errors.Wrap(err, "gitea client failed")
	}
	client.SetBasicAuth(username, password)

	c := &Client{
		Client:   client,
		Domain:   domain,
		URL:      giteaURL,
		Username: username,
		Password: password,
	}

	clientMemo = c

	return c, nil
}

// getMainDomain finds the main domain for Epinio
func getMainDomain(cluster *kubernetes.Cluster) (string, error) {
	// Get the ingress
	ingresses, err := cluster.ListIngress(deployments.GiteaDeploymentID, "app.kubernetes.io/name=gitea")
	if err != nil {
		return "", errors.Wrap(err, "failed to list ingresses for gitea")
	}

	if len(ingresses.Items) < 1 {
		return "", errors.New("gitea ingress not found")
	}

	if len(ingresses.Items) > 1 {
		return "", errors.New("more than one gitea ingress found")
	}

	if len(ingresses.Items[0].Spec.Rules) < 1 {
		return "", errors.New("gitea ingress has no rules")
	}

	if len(ingresses.Items[0].Spec.Rules) > 1 {
		return "", errors.New("gitea ingress has more than on rule")
	}

	host := ingresses.Items[0].Spec.Rules[0].Host

	return strings.TrimPrefix(host, "gitea."), nil
}

// getGiteaURL finds the URL for gitea
func getGiteaURL(config *config.Config, cluster *kubernetes.Cluster) (string, error) {
	// Get the ingress
	ingresses, err := cluster.ListIngress(deployments.GiteaDeploymentID, "app.kubernetes.io/name=gitea")
	if err != nil {
		return "", errors.Wrap(err, "failed to list ingresses for gitea")
	}

	if len(ingresses.Items) < 1 {
		return "", errors.New("gitea ingress not found")
	}

	if len(ingresses.Items) > 1 {
		return "", errors.New("more than one gitea ingress found")
	}

	if len(ingresses.Items[0].Spec.Rules) < 1 {
		return "", errors.New("gitea ingress has no rules")
	}

	if len(ingresses.Items[0].Spec.Rules) > 1 {
		return "", errors.New("gitea ingress has more than on rule")
	}

	host := ingresses.Items[0].Spec.Rules[0].Host

	return fmt.Sprintf("%s://%s", config.GiteaProtocol, host), nil
}

// getGiteaCredentials resolves Gitea's credentials
func getGiteaCredentials(cluster *kubernetes.Cluster) (string, string, error) {
	s, err := cluster.GetSecret(deployments.TektonStagingNamespace, GiteaCredentialsSecret)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to read gitea credentials")
	}

	username, ok := s.Data["username"]
	if !ok {
		return "", "", errors.Wrap(err, "username key not found in gitea credentials secret")
	}

	password, ok := s.Data["password"]
	if !ok {
		return "", "", errors.Wrap(err, "password key not found in gitea credentials secret")
	}

	return string(username), string(password), nil
}

func (c *Client) DeleteRepo(org, repo string) error {
	_, err := c.Client.DeleteRepo(org, repo)

	return err
}

func (c *Client) CreateOrg(org string) error {
	_, _, err := c.Client.CreateOrg(giteaSDK.CreateOrgOption{
		Name: org,
	})

	return err
}

func (c *Client) DeleteOrg(org string) error {
	_, err := c.Client.DeleteOrg(org)

	return err
}
