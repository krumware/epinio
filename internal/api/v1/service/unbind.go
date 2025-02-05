package service

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/configurationbinding"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Unbind handles the API endpoint /namespaces/:namespace/services/:service/unbind (POST)
// It removes the binding between the specified service and application
func (ctr Controller) Unbind(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx).WithName("Bind")

	namespace := c.Param("namespace")
	serviceName := c.Param("service")

	var bindRequest models.ServiceUnbindRequest
	err := c.BindJSON(&bindRequest)
	if err != nil {
		return apierror.BadRequest(err)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("looking for application")
	app, err := application.Lookup(ctx, cluster, namespace, bindRequest.AppName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if app == nil {
		return apierror.AppIsNotKnown(bindRequest.AppName)
	}

	apiErr := ValidateService(ctx, cluster, logger, namespace, serviceName)
	if apiErr != nil {
		return apiErr
	}

	// A service has one or more associated secrets containing its attributes. On
	// binding adding a specific set of labels turned these secrets into valid epinio
	// configurations. Here these configurations are simply unbound from the
	// application.

	logger.Info("looking for service secrets")

	serviceConfigurations, err := configurations.ForService(ctx, cluster, namespace, serviceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info(fmt.Sprintf("configurationSecrets found %+v\n", serviceConfigurations))

	username := requestctx.User(ctx).Username

	apiErr = UnbindService(ctx, cluster, logger, namespace, app.AppRef().Name, username, serviceConfigurations)
	if apiErr != nil {
		return apiErr // already apierror.MultiError
	}

	response.OK(c)
	return nil
}

func UnbindService(
	ctx context.Context, cluster *kubernetes.Cluster, logger logr.Logger,
	namespace, appName, userName string,
	serviceConfigurations []v1.Secret,
) apierror.APIErrors {
	logger.Info("unbinding service configurations")

	for _, secret := range serviceConfigurations {
		// TODO: Don't `helm upgrade` after each removal. Do it once.
		errors := configurationbinding.DeleteBinding(
			ctx, cluster, namespace, appName, secret.Name, userName,
		)
		if errors != nil {
			return apierror.NewMultiError(errors.Errors())
		}
	}

	logger.Info("unbound service configurations")
	return nil
}
