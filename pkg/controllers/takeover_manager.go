package controllers

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-application-networking-k8s/pkg/aws"
	"github.com/aws/aws-application-networking-k8s/pkg/aws/services"
	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	k8sutils "github.com/aws/aws-application-networking-k8s/pkg/utils"
	"github.com/aws/aws-application-networking-k8s/pkg/utils/gwlog"
)

// TakeoverManager handles the takeover logic for VPC Lattice services
type TakeoverManager interface {
	// AttemptTakeover tries to take over ownership of a VPC Lattice service
	// Returns true if takeover was successful or not needed, false if failed
	// Supports all route types: HTTPRoute, GRPCRoute, TLSRoute
	AttemptTakeover(ctx context.Context, route core.Route, sourceControllerID string) (bool, error)

	// ValidateTakeoverAnnotation validates the takeover annotation is not empty
	// Performs basic validation to ensure the annotation value is present
	ValidateTakeoverAnnotation(annotation string) error

	// IsRetryableError determines if an error should trigger retry logic
	IsRetryableError(err error) bool
}

// defaultTakeoverManager implements the TakeoverManager interface
type defaultTakeoverManager struct {
	log    gwlog.Logger
	cloud  aws.Cloud
	client client.Client
}

// NewTakeoverManager creates a new TakeoverManager instance
func NewTakeoverManager(log gwlog.Logger, cloud aws.Cloud, client client.Client) TakeoverManager {
	return &defaultTakeoverManager{
		log:    log,
		cloud:  cloud,
		client: client,
	}
}

// ValidateTakeoverAnnotation validates that the takeover annotation is not empty
func (tm *defaultTakeoverManager) ValidateTakeoverAnnotation(annotation string) error {
	// Check if annotation is empty or contains only whitespace
	trimmed := strings.TrimSpace(annotation)
	if trimmed == "" {
		return fmt.Errorf("takeover annotation 'allow-takeover-from' cannot be empty or contain only whitespace")
	}

	// Additional validation: ensure it's not just special characters
	if len(trimmed) == 0 {
		return fmt.Errorf("takeover annotation 'allow-takeover-from' must contain valid controller identifier")
	}

	return nil
}

// AttemptTakeover attempts to take over ownership of an existing VPC Lattice service
func (tm *defaultTakeoverManager) AttemptTakeover(ctx context.Context, route core.Route, sourceControllerID string) (bool, error) {
	serviceName := k8sutils.LatticeServiceName(route.Name(), route.Namespace())

	tm.log.Infow(ctx, "takeover attempt started",
		"route", route.Name(),
		"namespace", route.Namespace(),
		"sourceController", sourceControllerID,
		"targetService", serviceName)

	// Find existing VPC Lattice service
	service, err := tm.cloud.Lattice().FindService(ctx, serviceName)
	if err != nil {
		if services.IsNotFoundError(err) {
			tm.log.Infow(ctx, "service not found, proceeding with normal creation",
				"route", route.Name(),
				"serviceName", serviceName)
			return true, nil
		}
		tm.log.Errorw(ctx, "failed to find service during takeover attempt",
			"route", route.Name(),
			"serviceName", serviceName,
			"error", err)
		return false, fmt.Errorf("failed to find service %s: %w", serviceName, err)
	}

	if service == nil || service.Arn == nil {
		tm.log.Infow(ctx, "service not found, proceeding with normal creation",
			"route", route.Name(),
			"serviceName", serviceName)
		return true, nil
	}

	serviceArn := *service.Arn
	tm.log.Infow(ctx, "found existing service for takeover",
		"route", route.Name(),
		"serviceName", serviceName,
		"serviceArn", serviceArn)

	// Get current ManagedBy tag
	currentManagedBy, err := tm.cloud.GetManagedByTag(ctx, serviceArn)
	if err != nil {
		tm.log.Errorw(ctx, "failed to get ManagedBy tag during takeover",
			"route", route.Name(),
			"serviceArn", serviceArn,
			"error", err)
		return false, fmt.Errorf("failed to get ManagedBy tag for service %s: %w", serviceArn, err)
	}

	tm.log.Infow(ctx, "retrieved current ManagedBy tag",
		"route", route.Name(),
		"serviceArn", serviceArn,
		"currentManagedBy", currentManagedBy,
		"sourceController", sourceControllerID)

	// Compare source controller ID with current ManagedBy value
	if currentManagedBy != sourceControllerID {
		tm.log.Warnw(ctx, "source controller mismatch, proceeding with normal creation",
			"route", route.Name(),
			"serviceArn", serviceArn,
			"expectedSource", sourceControllerID,
			"actualManagedBy", currentManagedBy)
		return true, nil
	}

	// Update ManagedBy tag to claim ownership
	newManagedBy := tm.cloud.Config().AccountId + "/" + tm.cloud.Config().ClusterName + "/" + tm.cloud.Config().VpcId
	err = tm.cloud.UpdateManagedByTag(ctx, serviceArn, newManagedBy)
	if err != nil {
		tm.log.Errorw(ctx, "failed to update ManagedBy tag during takeover",
			"route", route.Name(),
			"serviceArn", serviceArn,
			"oldManagedBy", currentManagedBy,
			"newManagedBy", newManagedBy,
			"error", err)
		return false, fmt.Errorf("failed to update ManagedBy tag for service %s: %w", serviceArn, err)
	}

	tm.log.Infow(ctx, "takeover completed successfully",
		"route", route.Name(),
		"serviceArn", serviceArn,
		"oldManagedBy", currentManagedBy,
		"newManagedBy", newManagedBy)

	return true, nil
}

// IsRetryableError determines if an error should trigger retry logic
func (tm *defaultTakeoverManager) IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for AWS service errors that are typically retryable
	errorStr := strings.ToLower(err.Error())

	// Retryable AWS errors
	retryableErrors := []string{
		"throttling",
		"rate exceeded",
		"service unavailable",
		"internal error",
		"timeout",
		"connection reset",
		"connection refused",
	}

	for _, retryableError := range retryableErrors {
		if strings.Contains(errorStr, retryableError) {
			return true
		}
	}

	return false
}
