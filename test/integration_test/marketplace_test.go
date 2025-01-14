//go:build integrationtest
// +build integrationtest

package integrationtest

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-version"
	corev1 "github.com/libopenstorage/operator/pkg/apis/core/v1"
	testutil "github.com/libopenstorage/operator/pkg/util/test"
	"github.com/libopenstorage/operator/test/integration_test/types"
	ci_utils "github.com/libopenstorage/operator/test/integration_test/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const (
	clusterName = "test-stc"
)

var marketplaceOperatorTestCases = []types.TestCase{
	{
		TestName:        "BasicInstallViaOcpMarketplace",
		TestrailCaseIDs: []string{""},
		TestSpec: func(t *testing.T) interface{} {
			// Create PX vSphere secret if provisioning PX on vSphere based OCP
			if ci_utils.IsOcp && ci_utils.CloudProvider == "vsphere" {
				if ci_utils.PxVsphereUsername == "" || ci_utils.PxVspherePassword == "" {
					require.NoError(t, fmt.Errorf("Unable to create Portworx vSPhere secret because --portworx-vsphere-username or --portworx-vsphere-password are not passed"))
				}
				err := ci_utils.CreatePxVsphereSecret(ci_utils.PxNamespace, ci_utils.PxVsphereUsername, ci_utils.PxVspherePassword)
				require.NoError(t, err)
			}
			// Construct StorageCluster
			cluster := &corev1.StorageCluster{}
			cluster.Name = clusterName
			err := ci_utils.ConstructStorageCluster(cluster, ci_utils.PxSpecGenURL, ci_utils.PxSpecImages)
			require.NoError(t, err)
			return cluster
		},
		TestFunc: BasicInstallViaOcpMarketplace,
	},
	{
		TestName:        "BasicUpgradeOperatorViaOcpMarketplace",
		TestrailCaseIDs: []string{""},
		TestSpec: func(t *testing.T) interface{} {
			// Create PX vSphere secret if provisioning PX on vSphere based OCP
			if ci_utils.IsOcp && ci_utils.CloudProvider == "vsphere" {
				if ci_utils.PxVsphereUsername == "" || ci_utils.PxVspherePassword == "" {
					require.NoError(t, fmt.Errorf("Unable to create Portworx vSPhere secret because --portworx-vsphere-username or --portworx-vsphere-password are not passed"))
				}
				err := ci_utils.CreatePxVsphereSecret(ci_utils.PxNamespace, ci_utils.PxVsphereUsername, ci_utils.PxVspherePassword)
				require.NoError(t, err)
			}

			// Construct StorageCluster
			cluster := &corev1.StorageCluster{}
			cluster.Name = clusterName
			err := ci_utils.ConstructStorageCluster(cluster, ci_utils.PxSpecGenURL, ci_utils.PxSpecImages)
			require.NoError(t, err)
			return cluster
		},
		ShouldSkip: func(tc *types.TestCase) bool {
			if len(ci_utils.OperatorUpgradeHopsImageList) == 0 {
				logrus.Warnf("--operator-upgrade-hops-image-list is empty, cannot run BasicUpgradeOperatorViaOcpMarketplace test")
				return true
			}
			return ci_utils.PxOperatorVersion.LessThan(ci_utils.PxOperatorVer1_7)
		},
		TestFunc: BasicUpgradeOperatorViaOcpMarketplace,
	},
}

func TestMarketplaceOperator(t *testing.T) {
	for _, tc := range marketplaceOperatorTestCases {
		tc.ShouldSkip = shouldSkipMarketplaceOperatorTests
		tc.RunTest(t)
	}
}

func shouldSkipMarketplaceOperatorTests(tc *types.TestCase) bool {
	return !ci_utils.IsOcp
}

func BasicInstallViaOcpMarketplace(tc *types.TestCase) func(*testing.T) {
	return func(t *testing.T) {
		// Get the StorageCluster to start with
		testSpec := tc.TestSpec(t)
		cluster, ok := testSpec.(*corev1.StorageCluster)
		require.True(t, ok)

		// Deploy PX Operator via OCP Marketplace
		err := ci_utils.DeployAndValidatePxOperatorViaMarketplace(ci_utils.PxOperatorTag)
		require.NoError(t, err)

		// Create and validate StorageCluster
		cluster = ci_utils.DeployAndValidateStorageCluster(cluster, ci_utils.PxSpecImages, t)

		// Delete and validate StorageCluster deletion
		ci_utils.UninstallAndValidateStorageCluster(cluster, t)

		// Delete and validate PX Operator via OCP Marketplace deletion
		err = ci_utils.DeleteAndValidatePxOperatorViaMarketplace()
		require.NoError(t, err)
	}
}

func BasicUpgradeOperatorViaOcpMarketplace(tc *types.TestCase) func(*testing.T) {
	return func(t *testing.T) {
		// Get the StorageCluster to start with
		testSpec := tc.TestSpec(t)
		cluster, ok := testSpec.(*corev1.StorageCluster)
		require.True(t, ok)

		// Loop through PX Operator version list and perform upgrades
		for hop, hopImage := range ci_utils.OperatorUpgradeHopsImageList {
			operatorVersionTagHop := strings.Split(hopImage, ":")[len(strings.Split(hopImage, ":"))-1]
			operatorVersionHop, _ := version.NewVersion(operatorVersionTagHop)

			// Deploy PX Operator and StorageCluster on the first hop only
			if hop == 0 {
				logrus.Infof("Deploying starting PX Operator [%s]", operatorVersionHop.String())
				// Deploy PX Operator via OCP Marketplace
				err := ci_utils.DeployAndValidatePxOperatorViaMarketplace(operatorVersionTagHop)
				require.NoError(t, err)

				// Deploy and validate StorageCluster
				logrus.Infof("Deploying StorageCluster once [%s]", cluster.Name)
				cluster = ci_utils.DeployAndValidateStorageCluster(cluster, ci_utils.PxSpecImages, t)
			}

			// Validate PX Operator deployment
			_, err := ci_utils.ValidatePxOperator(ci_utils.PxNamespace)
			require.NoError(t, err)

			// Get PX Operator version
			pxOperatorCurrentVersion, err := ci_utils.GetPXOperatorVersion()
			require.NoError(t, err)
			logrus.Infof("Current PX Operator version [%s]", pxOperatorCurrentVersion.String())

			// Get PX Operator tag from the OperatorUpgradeHopsImageList and check if its same PX Operator version that is currently installed
			if operatorVersionHop.LessThanOrEqual(pxOperatorCurrentVersion) {
				logrus.Infof("Skipping upgrade of PX Operator from [%s] to [%s], shouldn't upgrade to same or lower PX Operator version", pxOperatorCurrentVersion.String(), operatorVersionHop.String())
				continue
			}

			// Upgrade PX Operator to the next hop
			logrus.Infof("Upgrading PX Operator from [%s] to [%s]", pxOperatorCurrentVersion.String(), operatorVersionHop.String())
			ci_utils.UpdateAndValidatePxOperatorViaMarketplace(operatorVersionTagHop)
			require.NoError(t, err)

			// Sleep for 15 secs to let cluster stabilize
			logrus.Infof("Upgraded PX Operator from [%s] to [%s], letting it sleep for 15 secs to stabilize and let make changes to StorageCluster and/or existing objects", pxOperatorCurrentVersion.String(), operatorVersionHop.String())
			time.Sleep(15 * time.Second)

			// Validate StorageCluster
			err = testutil.ValidateStorageCluster(ci_utils.PxSpecImages, cluster, ci_utils.DefaultValidateDeployTimeout, ci_utils.DefaultValidateDeployRetryInterval, true, "")
			require.NoError(t, err)
		}

		// Delete and validate StorageCluster deletion
		ci_utils.UninstallAndValidateStorageCluster(cluster, t)

		// Delete and validate PX Operator via OCP Marketplace deletion
		err := ci_utils.DeleteAndValidatePxOperatorViaMarketplace()
		require.NoError(t, err)
	}
}
