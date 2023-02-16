package build

import (
	"fmt"
	"github.com/itura/fun/pkg/fun"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseConfig(t *testing.T) {
	builder := NewTestBuilder()
	cases := []struct {
		args     ActionArgs
		name     string
		expected PipelineConfig
	}{
		{
			name:     "ValidPipelineConfig",
			args:     TestArgs("test_fixtures/valid_pipeline_config.yaml"),
			expected: ValidPipelineConfig(builder),
		},
		{
			name: "InvalidSecretName",
			args: TestArgs("test_fixtures/invalid_secret_name.yaml"),
			expected: FailedParse("My Build", NewValidationErrors("applications").
				PutChild(NewValidationErrors("db").
					PutChild(NewValidationErrors("secrets").
						Put("postgresql.auth.postgresPassword", fmt.Errorf("secret 'beepboop' not configured in any secretProvider")),
					),
				),
			),
		}, {
			name: "InvalidSecretProvider",
			args: TestArgs("test_fixtures/invalid_secret_provider.yaml"),
			expected: FailedParse("My Build", NewValidationErrors("").
				PutChild(NewValidationErrors("resources").
					PutChild(NewValidationErrors("secretProviders").
						PutChild(NewValidationErrors("0").
							Put("id", eMissingRequiredField).
							Put("secretNames", eMissingRequiredField),
						).
						PutChild(NewValidationErrors("1").
							Put("secretNames", eMissingRequiredField).
							PutChild(NewValidationErrors("config").
								Put("project", eMissingRequiredField)),
						).
						PutChild(NewValidationErrors("2").
							Put("type", eMissingRequiredField),
						).
						PutChild(NewValidationErrors("3").
							Put("secretNames", eMissingRequiredField).
							Put("config", eMissingRequiredField),
						),
					),
				),
			),
		},
		{
			name:     "InvalidSecretProviderType",
			args:     TestArgs("test_fixtures/invalid_secret_provider_type.yaml"),
			expected: FailedParse("", SecretProviderTypeEnum.InvalidEnumValue("aws")),
		},
		{
			name: "InvalidCloudProvider",
			args: TestArgs("test_fixtures/invalid_cloud_provider.yaml"),
			expected: FailedParse("My Build", NewValidationErrors("").
				PutChild(NewValidationErrors("resources").
					Put("cloudProvider", eMissingRequiredField).
					Put("kubernetesCluster", eMissingRequiredField).
					PutChild(NewValidationErrors("artifactRepository").
						Put("host", eMissingRequiredField)),
				)),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseConfig(tc.args, NewAlwaysChanged())
			assert.Equal(t, tc.expected.BuildName, result.BuildName)
			assert.Equal(t, tc.expected.Artifacts, result.Artifacts)
			assert.Equal(t, tc.expected.Applications, result.Applications)
			fmt.Println(result.Error)
			assert.Equal(t, tc.expected.Error, result.Error)
		})
	}
}

func TestGithubActionsGeneration(t *testing.T) {
	result, err := ParseConfigForGeneration("test_fixtures/valid_pipeline_config.yaml", "???")
	assert.Nil(t, err)
	//assert.Equal(t, "yeehaw", result)
	_ = result.WriteYaml("test_fixtures/yeehaw.yaml")
}

func TestCloudProviderValidations(t *testing.T) {
	cp := CloudProviderConfig{
		Type: cloudProviderTypeGcp,
		Config: fun.NewConfig[string]().
			Set("serviceAccount", "yeehaw@yahoo.com").
			Set("workloadIdentityProvider", "it me"),
	}

	errs := cp.Validate("cloudProvider")
	assert.Equal(t, false, errs.IsPresent())
	assert.Equal(t,
		NewValidationErrors("cloudProvider"),
		errs,
	)

	cp = CloudProviderConfig{
		Type:   cloudProviderTypeGcp,
		Config: fun.NewConfig[string](),
	}

	errs = cp.Validate("cloudProvider")
	assert.Equal(t, true, errs.IsPresent())
	assert.Equal(t,
		NewValidationErrors("cloudProvider").
			PutChild(NewValidationErrors("config").
				Put("serviceAccount", CloudProviderMissingField("gcp")).
				Put("workloadIdentityProvider", CloudProviderMissingField("gcp"))),
		errs,
	)

	cp = CloudProviderConfig{
		Type: cloudProviderTypeGcp,
		Config: fun.NewConfig[string]().
			Set("serviceAccount", "yeehaw@yahoo.com"),
	}
	errs = cp.Validate("cloudProvider")
	assert.Equal(t, true, errs.IsPresent())
	assert.Equal(t,
		NewValidationErrors("cloudProvider").
			PutChild(NewValidationErrors("config").
				Put("workloadIdentityProvider", CloudProviderMissingField("gcp"))),
		errs,
	)
}

func TestResourcesValidation(t *testing.T) {
	resources := Resources{
		SecretProviders: SecretProviderConfigs{
			SecretProviderConfig{
				Id:     "github",
				Type:   secretProviderTypeGithub,
				Config: nil,
				SecretNames: []string{
					"yeehaw",
				},
			},
			SecretProviderConfig{
				Id:   "gcp-cool-proj",
				Type: secretProviderTypeGcp,
				Config: fun.Config[string]{
					"project": "cool-proj",
				},
				SecretNames: []string{
					"hoowee",
				},
			},
		},
		CloudProvider: CloudProviderConfig{
			Type: cloudProviderTypeGcp,
			Config: fun.NewConfig[string]().
				Set("serviceAccount", "yeehaw@yahoo.com").
				Set("workloadIdentityProvider", "it me"),
		},
		ArtifactRepository: ArtifactRepository{
			Host: "us",
			Name: "repo",
		},
		KubernetesCluster: ClusterConfig{
			Name:     "cluster",
			Location: "new zealand",
		},
	}

	errs := resources.Validate("resources")
	assert.Equal(t, false, errs.IsPresent())

	resources = Resources{
		SecretProviders: SecretProviderConfigs{
			SecretProviderConfig{
				Id:   "gcp-cool-proj",
				Type: secretProviderTypeGcp,
				Config: fun.Config[string]{
					"project": "cool-proj",
				},
			},
			SecretProviderConfig{},
		},
		CloudProvider: CloudProviderConfig{
			Type: cloudProviderTypeGcp,
			Config: fun.NewConfig[string]().
				Set("serviceAccount", "yeehaw@yahoo.com"),
		},
		ArtifactRepository: ArtifactRepository{
			Host: "us",
			Name: "repo",
		},
		KubernetesCluster: ClusterConfig{
			Name:     "cluster",
			Location: "new zealand",
		},
	}

	errs = resources.Validate("resources")
	assert.Equal(t, true, errs.IsPresent())
	//assert.Equal(t,
	//	NewValidationErrors("secretProviders"),
	//	errs,
	//)
	fmt.Println(errs.Error())
}

func TestValidateTags(t *testing.T) {

}
