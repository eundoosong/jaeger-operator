// +build self_provisioned_elasticsearch_kafka

package e2e

import (
	goctx "context"
	"testing"

	kafkav1beta1 "github.com/jaegertracing/jaeger-operator/pkg/apis/kafka/v1beta1"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jaegertracing/jaeger-operator/pkg/apis"
	v1 "github.com/jaegertracing/jaeger-operator/pkg/apis/jaegertracing/v1"
	esv1 "github.com/jaegertracing/jaeger-operator/pkg/storage/elasticsearch/v1"
)

type SelfProvisionedTestSuite struct {
	suite.Suite
}

func (suite *SelfProvisionedTestSuite) SetupSuite() {
	t = suite.T()
	if !isOpenShift(t) {
		t.Skipf("Test %s is currently supported only on OpenShift because es-operator runs only on OpenShift\n", t.Name())
	}

	assert.NoError(t, framework.AddToFrameworkScheme(apis.AddToScheme, &v1.JaegerList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Jaeger",
			APIVersion: "jaegertracing.io/v1",
		},
	}))
	assert.NoError(t, framework.AddToFrameworkScheme(apis.AddToScheme, &esv1.ElasticsearchList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Elasticsearch",
			APIVersion: "logging.openshift.io/v1",
		},
	}))
	assert.NoError(t, framework.AddToFrameworkScheme(apis.AddToScheme, &kafkav1beta1.KafkaList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Kafka",
			APIVersion: "kafka.strimzi.io/v1beta1",
		},
	}))

	var err error
	ctx, err = prepare(t)
	if err != nil {
		if ctx != nil {
			ctx.Cleanup()
		}
		require.FailNow(t, "Failed in prepare")
	}
	fw = framework.Global
	namespace, _ = ctx.GetNamespace()
	require.NotNil(t, namespace, "GetNamespace failed")
}

func (suite *SelfProvisionedTestSuite) TearDownSuite() {
	handleSuiteTearDown()
}

func TestSelfProvisionedSuite(t *testing.T) {
	suite.Run(t, new(SelfProvisionedTestSuite))
}

func (suite *SelfProvisionedTestSuite) SetupTest() {
	t = suite.T()
}

func (suite *SelfProvisionedTestSuite) AfterTest(suiteName, testName string) {
	handleTestFailure()
}

func (suite *SelfProvisionedTestSuite) TestSelfProvisionedESAndKafkaSmokeTest() {
	// create jaeger custom resource
	jaegerInstanceName := "simple-prod"
	exampleJaeger := getJaegerSelfProvisionedESAndKafka(jaegerInstanceName)
	err := fw.Client.Create(goctx.TODO(), exampleJaeger, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	require.NoError(t, err, "Error deploying example Jaeger")
	defer undeployJaegerInstance(exampleJaeger)

	err = e2eutil.WaitForDeployment(t, fw.KubeClient, namespace, jaegerInstanceName+"-collector", 1, retryInterval, timeout*3)
	require.NoError(t, err, "Error waiting for collector deployment")

	err = e2eutil.WaitForDeployment(t, fw.KubeClient, namespace, jaegerInstanceName+"-query", 1, retryInterval, timeout)
	require.NoError(t, err, "Error waiting for query deployment")

	err = WaitForDeployment(t, fw.KubeClient, namespace, jaegerInstanceName+"-ingester", 1, retryInterval, timeout)
	require.NoError(t, err, "Error waiting for ingester deployment")

	ProductionSmokeTest(jaegerInstanceName)
}

func getJaegerSelfProvisionedESAndKafka(instanceName string) *v1.Jaeger {
	return &v1.Jaeger{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Jaeger",
			APIVersion: "jaegertracing.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceName,
			Namespace: namespace,
		},
		Spec: v1.JaegerSpec{
			Strategy: v1.DeploymentStrategyStreaming,
			Storage: v1.JaegerStorageSpec{
				Type: "elasticsearch",
				Elasticsearch: v1.ElasticsearchSpec{
					NodeCount: 1,
					Resources: &corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
						Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
					},
				},
			},
		},
	}
}
