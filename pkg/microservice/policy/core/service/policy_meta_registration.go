/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
)

type PolicyMeta struct {
	Resource    string        `json:"resource"`
	Alias       string        `json:"alias"`
	Description string        `json:"description"`
	Rules       []*PolicyRule `json:"rules"`
}

type PolicyRule struct {
	Action      string        `json:"action"`
	Alias       string        `json:"alias"`
	Description string        `json:"description"`
	Rules       []*ActionRule `json:"rules"`
}

type ActionRule struct {
	Method          string      `json:"method"`
	Endpoint        string      `json:"endpoint"`
	ResourceType    string      `json:"resourceType,omitempty"`
	IDRegex         string      `json:"idRegex,omitempty"`
	MatchAttributes []attribute `json:"matchAttributes,omitempty"`
}

type attribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type PolicyDefinition struct {
	Resource    string                  `json:"resource"`
	Alias       string                  `json:"alias"`
	Description string                  `json:"description"`
	Rules       []*PolicyRuleDefinition `json:"rules"`
}

type PolicyRuleDefinition struct {
	Action      string `json:"action"`
	Alias       string `json:"alias"`
	Description string `json:"description"`
}

func CreateOrUpdatePolicyRegistration(p *PolicyMeta, _ *zap.SugaredLogger) error {
	obj := &models.PolicyMeta{
		Resource:    p.Resource,
		Alias:       p.Alias,
		Description: p.Description,
	}

	for _, r := range p.Rules {
		rule := &models.PolicyMetaRule{
			Action:      r.Action,
			Alias:       r.Alias,
			Description: r.Description,
		}
		for _, ar := range r.Rules {
			var as []models.Attribute
			for _, a := range ar.MatchAttributes {
				as = append(as, models.Attribute{Key: a.Key, Value: a.Value})
			}

			rule.Rules = append(rule.Rules, &models.ActionRule{
				Method:          ar.Method,
				Endpoint:        ar.Endpoint,
				ResourceType:    ar.ResourceType,
				IDRegex:         ar.IDRegex,
				MatchAttributes: as,
			})
		}
		obj.Rules = append(obj.Rules, rule)
	}

	return mongodb.NewPolicyMetaColl().UpdateOrCreate(obj)
}

func GetPolicyRegistrationDefinitions(scope, envType string, _ *zap.SugaredLogger) ([]*PolicyDefinition, error) {
	policieMetas, err := mongodb.NewPolicyMetaColl().List()
	if err != nil {
		return nil, err
	}
	systemScopeSet := sets.NewString("TestCenter", "DataCenter", "Template", "DeliveryCenter")
	projectScopeSet := sets.NewString("Workflow", "Environment", "ProductionEnvironment", "Test", "Delivery", "Build", "Service")
	systemPolicyMetas, projectPolicyMetas, filteredPolicyMetas := []*models.PolicyMeta{}, []*models.PolicyMeta{}, []*models.PolicyMeta{}
	for _, v := range policieMetas {
		if systemScopeSet.Has(v.Resource) {
			systemPolicyMetas = append(systemPolicyMetas, v)
		} else if projectScopeSet.Has(v.Resource) {
			projectPolicyMetas = append(projectPolicyMetas, v)
		}
	}

	switch scope {
	case string(types.SystemScope):
		filteredPolicyMetas = systemPolicyMetas
	case string(types.ProjectScope):
		for i, meta := range projectPolicyMetas {
			if meta.Resource == "Environment" || meta.Resource == "ProductionEnvironment" {
				tmpRules := []*models.PolicyMetaRule{}
				for _, rule := range meta.Rules {
					if rule.Action == "debug_pod" && envType == setting.PMDeployType {
						continue
					}
					if rule.Action == "ssh_pm" && envType != setting.PMDeployType {
						continue
					}
					tmpRules = append(tmpRules, rule)
				}
				projectPolicyMetas[i].Rules = tmpRules
			}
		}
		filteredPolicyMetas = projectPolicyMetas
	default:
		filteredPolicyMetas = policieMetas
	}
	var res []*PolicyDefinition
	for _, meta := range filteredPolicyMetas {
		pd := &PolicyDefinition{
			Resource:    meta.Resource,
			Alias:       meta.Alias,
			Description: meta.Description,
		}
		for _, r := range meta.Rules {
			pd.Rules = append(pd.Rules, &PolicyRuleDefinition{
				Action:      r.Action,
				Alias:       r.Alias,
				Description: r.Description,
			})
		}
		res = append(res, pd)
	}
	return res, nil
}
