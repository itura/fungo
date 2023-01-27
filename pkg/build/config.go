package build

import (
	"fmt"
	"os"

	"github.com/itura/fun/pkg/fun"

	"gopkg.in/yaml.v3"
)

type Preset string

var (
	presetGolang Preset = "golang"
)

type ArtifactType string

var (
	typeLibGo ArtifactType = "lib-go"
	typeAppGo ArtifactType = "app-go"
	typeApp   ArtifactType = "app"
	typeLib   ArtifactType = "lib"
)

type ApplicationType string

var (
	typeHelm      ApplicationType = "helm"
	typeTerraform ApplicationType = "terraform"
)

type SecretProviderType string

var (
	typeGcp    SecretProviderType = "gcp"
	typeGithub SecretProviderType = "github-actions"
)

type ClusterConfig struct {
	Name     string
	Location string
}

type ArtifactRepository struct {
	Host string
	Name string
}

type CloudProvider struct {
	Type   string
	Config map[string]string
}

type SecretProvider struct {
	Type   SecretProviderType
	Config map[string]string
}

type SecretProviders fun.Config[SecretProvider]

type SecretConfig struct {
	HelmKey    string "yaml:\"helmKey\""
	SecretName string "yaml:\"secretName\""
	Provider   string
}

type PipelineConfig struct {
	Name      string
	Resources struct {
		ArtifactRepository ArtifactRepository `yaml:"artifactRepository"`
		KubernetesCluster  ClusterConfig      `yaml:"kubernetesCluster"`
		SecretProviders    SecretProviders    `yaml:"secretProviders"`
		CloudProvider      CloudProvider      `yaml:"cloudProvider"`
	}
	Artifacts []struct {
		Id           string
		Path         string
		Dependencies []string
		Type         ArtifactType
	}
	Applications []struct {
		Id           string
		Path         string
		Namespace    string
		Artifacts    []string
		Values       []HelmValue
		Secrets      []SecretConfig
		Dependencies []string
		Type         ApplicationType
	}
}

func _parsePipelineConfig(configPath string) (PipelineConfig, error) {
	dat, err := os.ReadFile(configPath)
	if err != nil {
		return PipelineConfig{}, err
	}

	var config PipelineConfig
	err = yaml.Unmarshal(dat, &config)
	if err != nil {
		return PipelineConfig{}, err
	}

	return config, nil
}

func parseConfig(args ActionArgs, previousSha string) ParsedConfig {
	config, err := _parsePipelineConfig(args.ConfigPath)
	if err != nil {
		return FailedParse("", err)
	}

	cloudProvider := config.Resources.CloudProvider

	if cloudProvider.Type == "gcp" {
		_, ok := cloudProvider.Config["serviceAccount"]
		if !ok {
			return FailedParse(config.Name, fmt.Errorf("No service account configured for cloud provider of type %s", cloudProvider.Type))
		}

		_, ok = cloudProvider.Config["workloadIdentityProvider"]
		if !ok {
			return FailedParse(config.Name, fmt.Errorf("No Workload Identity Provider configured for cloud provider of type %s", cloudProvider.Type))
		}
	} else {
		return FailedParse(config.Name, InvalidCloudProvider{"Missing/Unknown"})
	}

	var providerConfigs map[string]SecretProvider = config.Resources.SecretProviders

	// TODO extract
	for _, provider := range providerConfigs {
		if provider.Type != typeGcp && provider.Type != typeGithub {
			return FailedParse(config.Name, InvalidSecretProviderType{GivenType: string(provider.Type)})
		}
	}

	var repository string = fmt.Sprintf("%s/%s/%s", config.Resources.ArtifactRepository.Host, cloudProvider.Config["project"], config.Resources.ArtifactRepository.Name)

	artifacts := make(map[string]Artifact)
	for _, spec := range config.Artifacts {
		var upstreams []Job
		var cd ChangeDetection
		if args.Force {
			cd = NewAlwaysChanged()
		} else {
			_cd := NewGitChangeDetection(previousSha).
				AddPaths(spec.Path)

			// todo make agnostic to ordering
			for _, id := range spec.Dependencies {
				_cd = _cd.AddPaths(artifacts[id].Path)
				upstreams = append(upstreams, artifacts[id])
			}
			cd = _cd
		}

		artifacts[spec.Id] = Artifact{
			Type:            spec.Type,
			Id:              spec.Id,
			Path:            spec.Path,
			Project:         args.ProjectId,
			Repository:      repository,
			Host:            config.Resources.ArtifactRepository.Host,
			CurrentSha:      args.CurrentSha,
			hasDependencies: len(spec.Dependencies) > 0,
			Upstreams:       upstreams,
			hasChanged:      cd.HasChanged(),
			CloudProvider:   config.Resources.CloudProvider,
		}
	}

	applications := make(map[string]Application)
	for _, spec := range config.Applications {
		var upstreams []Job
		var cd ChangeDetection
		if args.Force {
			cd = NewAlwaysChanged()
		} else {
			_cd := NewGitChangeDetection(previousSha).
				AddPaths(spec.Path)

			// todo make agnostic to ordering
			for _, id := range spec.Artifacts {
				_cd = _cd.AddPaths(artifacts[id].Path)
				upstreams = append(upstreams, artifacts[id])
			}
			for _, id := range spec.Dependencies {
				_cd = _cd.AddPaths(applications[id].Path)
				upstreams = append(upstreams, applications[id])
			}
			cd = _cd
		}

		var secretConfigs = spec.Secrets

		helmSecretValues := make(map[string][]HelmSecretValue, len(secretConfigs))
		for _, secretConfig := range secretConfigs {
			_, ok := providerConfigs[secretConfig.Provider]

			if !ok {
				return FailedParse(config.Name, MissingSecretProvider{})
			}

			helmSecretValue := HelmSecretValue{
				HelmKey:    secretConfig.HelmKey,
				SecretName: secretConfig.SecretName,
			}

			providerSecretsList := helmSecretValues[secretConfig.Provider]
			helmSecretValues[secretConfig.Provider] = append(providerSecretsList, helmSecretValue)
		}

		hasDependencies := len(spec.Dependencies) > 0 || len(spec.Artifacts) > 0
		applications[spec.Id] = Application{
			Type:              spec.Type,
			Id:                spec.Id,
			Path:              spec.Path,
			ProjectId:         args.ProjectId,
			Repository:        repository,
			CurrentSha:        args.CurrentSha,
			Namespace:         spec.Namespace,
			Values:            spec.Values,
			Upstreams:         upstreams,
			hasDependencies:   hasDependencies,
			KubernetesCluster: config.Resources.KubernetesCluster,
			Secrets:           helmSecretValues,
			SecretProviders:   providerConfigs,
			hasChanged:        cd.HasChanged(),
			CloudProvider:     config.Resources.CloudProvider,
		}
	}

	return SuccessfulParse(config.Name, artifacts, applications)
}
