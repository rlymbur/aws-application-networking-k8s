package controllers

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/aws/aws-sdk-go/service/vpclattice"

	"github.com/aws/aws-application-networking-k8s/pkg/aws"
	"github.com/aws/aws-application-networking-k8s/pkg/aws/services"
	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	"github.com/aws/aws-application-networking-k8s/pkg/utils/gwlog"
)

func TestTakeoverManager_ValidateTakeoverAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		annotation  string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid annotation",
			annotation:  "source-controller-id",
			expectError: false,
		},
		{
			name:        "valid annotation with spaces",
			annotation:  "  source-controller-id  ",
			expectError: false,
		},
		{
			name:        "empty annotation",
			annotation:  "",
			expectError: true,
			errorMsg:    "takeover annotation 'allow-takeover-from' cannot be empty or contain only whitespace",
		},
		{
			name:        "whitespace only annotation",
			annotation:  "   ",
			expectError: true,
			errorMsg:    "takeover annotation 'allow-takeover-from' cannot be empty or contain only whitespace",
		},
		{
			name:        "tab and newline annotation",
			annotation:  "\t\n",
			expectError: true,
			errorMsg:    "takeover annotation 'allow-takeover-from' cannot be empty or contain only whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := &defaultTakeoverManager{}
			err := tm.ValidateTakeoverAnnotation(tt.annotation)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTakeoverManager_AttemptTakeover(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCloud := aws.NewMockCloud(ctrl)
	mockLattice := services.NewMockLattice(ctrl)

	// Create a fake client for testing
	k8sScheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(k8sScheme)
	gwv1.Install(k8sScheme)
	k8sClient := testclient.NewClientBuilder().WithScheme(k8sScheme).Build()

	logger := gwlog.FallbackLogger

	tm := NewTakeoverManager(logger, mockCloud, k8sClient)

	ctx := context.Background()
	route := core.NewHTTPRoute(gwv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "test-namespace",
		},
	})
	sourceControllerID := "source-controller-123"
	serviceName := "test-route-test-namespace"
	serviceArn := "arn:aws:vpc-lattice:us-west-2:123456789012:service/svc-123"

	tests := []struct {
		name           string
		setupMocks     func()
		expectedResult bool
		expectError    bool
		errorContains  string
	}{
		{
			name: "service not found - proceed with creation",
			setupMocks: func() {
				mockCloud.EXPECT().Lattice().Return(mockLattice)
				mockLattice.EXPECT().FindService(ctx, serviceName).Return(nil, services.NewNotFoundError("service", serviceName))
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "service found but nil - proceed with creation",
			setupMocks: func() {
				mockCloud.EXPECT().Lattice().Return(mockLattice)
				mockLattice.EXPECT().FindService(ctx, serviceName).Return(nil, nil)
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "successful takeover - source matches",
			setupMocks: func() {
				service := &vpclattice.ServiceSummary{
					Arn: &serviceArn,
				}
				mockCloud.EXPECT().Lattice().Return(mockLattice)
				mockLattice.EXPECT().FindService(ctx, serviceName).Return(service, nil)
				mockCloud.EXPECT().GetManagedByTag(ctx, serviceArn).Return(sourceControllerID, nil)

				// Mock cloud config for new ManagedBy tag
				config := aws.CloudConfig{
					AccountId:   "123456789012",
					ClusterName: "test-cluster",
					VpcId:       "vpc-123",
				}
				mockCloud.EXPECT().Config().Return(config).AnyTimes()
				newManagedBy := "123456789012/test-cluster/vpc-123"
				mockCloud.EXPECT().UpdateManagedByTag(ctx, serviceArn, newManagedBy).Return(nil)
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "source controller mismatch - proceed with creation",
			setupMocks: func() {
				service := &vpclattice.ServiceSummary{
					Arn: &serviceArn,
				}
				mockCloud.EXPECT().Lattice().Return(mockLattice)
				mockLattice.EXPECT().FindService(ctx, serviceName).Return(service, nil)
				mockCloud.EXPECT().GetManagedByTag(ctx, serviceArn).Return("different-controller", nil)
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "error finding service",
			setupMocks: func() {
				mockCloud.EXPECT().Lattice().Return(mockLattice)
				mockLattice.EXPECT().FindService(ctx, serviceName).Return(nil, errors.New("AWS API error"))
			},
			expectedResult: false,
			expectError:    true,
			errorContains:  "failed to find service",
		},
		{
			name: "error getting ManagedBy tag",
			setupMocks: func() {
				service := &vpclattice.ServiceSummary{
					Arn: &serviceArn,
				}
				mockCloud.EXPECT().Lattice().Return(mockLattice)
				mockLattice.EXPECT().FindService(ctx, serviceName).Return(service, nil)
				mockCloud.EXPECT().GetManagedByTag(ctx, serviceArn).Return("", errors.New("tag retrieval error"))
			},
			expectedResult: false,
			expectError:    true,
			errorContains:  "failed to get ManagedBy tag",
		},
		{
			name: "error updating ManagedBy tag",
			setupMocks: func() {
				service := &vpclattice.ServiceSummary{
					Arn: &serviceArn,
				}
				mockCloud.EXPECT().Lattice().Return(mockLattice)
				mockLattice.EXPECT().FindService(ctx, serviceName).Return(service, nil)
				mockCloud.EXPECT().GetManagedByTag(ctx, serviceArn).Return(sourceControllerID, nil)

				config := aws.CloudConfig{
					AccountId:   "123456789012",
					ClusterName: "test-cluster",
					VpcId:       "vpc-123",
				}
				mockCloud.EXPECT().Config().Return(config).AnyTimes()
				newManagedBy := "123456789012/test-cluster/vpc-123"
				mockCloud.EXPECT().UpdateManagedByTag(ctx, serviceArn, newManagedBy).Return(errors.New("tag update error"))
			},
			expectedResult: false,
			expectError:    true,
			errorContains:  "failed to update ManagedBy tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()

			result, err := tm.AttemptTakeover(ctx, route, sourceControllerID)

			assert.Equal(t, tt.expectedResult, result)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTakeoverManager_IsRetryableError(t *testing.T) {
	tm := &defaultTakeoverManager{}

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "throttling error",
			err:      errors.New("Throttling: Rate exceeded"),
			expected: true,
		},
		{
			name:     "service unavailable error",
			err:      errors.New("Service Unavailable"),
			expected: true,
		},
		{
			name:     "internal error",
			err:      errors.New("Internal Error occurred"),
			expected: true,
		},
		{
			name:     "timeout error",
			err:      errors.New("Request timeout"),
			expected: true,
		},
		{
			name:     "connection reset error",
			err:      errors.New("Connection reset by peer"),
			expected: true,
		},
		{
			name:     "connection refused error",
			err:      errors.New("Connection refused"),
			expected: true,
		},
		{
			name:     "permission denied error - not retryable",
			err:      errors.New("Access Denied"),
			expected: false,
		},
		{
			name:     "validation error - not retryable",
			err:      errors.New("Invalid parameter"),
			expected: false,
		},
		{
			name:     "not found error - not retryable",
			err:      errors.New("Resource not found"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.IsRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewTakeoverManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCloud := aws.NewMockCloud(ctrl)

	// Create a fake client for testing
	k8sScheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(k8sScheme)
	gwv1.Install(k8sScheme)
	k8sClient := testclient.NewClientBuilder().WithScheme(k8sScheme).Build()

	logger := gwlog.FallbackLogger

	tm := NewTakeoverManager(logger, mockCloud, k8sClient)

	assert.NotNil(t, tm)
	assert.IsType(t, &defaultTakeoverManager{}, tm)

	// Verify the implementation has the expected fields
	impl := tm.(*defaultTakeoverManager)
	assert.Equal(t, logger, impl.log)
	assert.Equal(t, mockCloud, impl.cloud)
	assert.Equal(t, k8sClient, impl.client)
}
