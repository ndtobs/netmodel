package exporter

import (
	"context"
	"strings"

	"github.com/ndtobs/netmodel/internal/gnmi"
	"github.com/ndtobs/netmodel/internal/model"
)

// RoutingPolicyExporter exports routing policy configuration
type RoutingPolicyExporter struct {
	routingPolicy *model.RoutingPolicy
}

func (e *RoutingPolicyExporter) Name() string {
	return "routing_policy"
}

func (e *RoutingPolicyExporter) Export(ctx context.Context, client *gnmi.Client) error {
	e.routingPolicy = &model.RoutingPolicy{}

	// Get routing policy config
	data, err := client.GetJSON(ctx, "/routing-policy")
	if err != nil {
		return err
	}

	if data == nil {
		return nil
	}

	// Parse defined-sets
	if definedSets, ok := data["openconfig-routing-policy:defined-sets"].(map[string]interface{}); ok {
		e.parseDefinedSets(definedSets)
	} else if definedSets, ok := data["defined-sets"].(map[string]interface{}); ok {
		e.parseDefinedSets(definedSets)
	}

	// Parse policy-definitions
	if policyDefs, ok := data["openconfig-routing-policy:policy-definitions"].(map[string]interface{}); ok {
		e.parsePolicyDefinitions(policyDefs)
	} else if policyDefs, ok := data["policy-definitions"].(map[string]interface{}); ok {
		e.parsePolicyDefinitions(policyDefs)
	}

	return nil
}

func (e *RoutingPolicyExporter) parseDefinedSets(data map[string]interface{}) {
	e.routingPolicy.DefinedSets = &model.DefinedSets{}

	// Parse prefix-sets
	if prefixSets, ok := data["openconfig-routing-policy:prefix-sets"].(map[string]interface{}); ok {
		e.parsePrefixSets(prefixSets)
	} else if prefixSets, ok := data["prefix-sets"].(map[string]interface{}); ok {
		e.parsePrefixSets(prefixSets)
	}

	// Parse community-sets (BGP-specific)
	if bgpSets, ok := data["openconfig-bgp-policy:bgp-defined-sets"].(map[string]interface{}); ok {
		e.parseBGPDefinedSets(bgpSets)
	} else if bgpSets, ok := data["bgp-defined-sets"].(map[string]interface{}); ok {
		e.parseBGPDefinedSets(bgpSets)
	}
}

func (e *RoutingPolicyExporter) parsePrefixSets(data map[string]interface{}) {
	var prefixSetList []interface{}

	if sets, ok := data["prefix-set"].([]interface{}); ok {
		prefixSetList = sets
	}

	for _, ps := range prefixSetList {
		psData, ok := ps.(map[string]interface{})
		if !ok {
			continue
		}

		prefixSet := model.PrefixSet{}

		if name, ok := psData["name"].(string); ok {
			prefixSet.Name = name
		}

		// Get config
		if config, ok := psData["config"].(map[string]interface{}); ok {
			if mode, ok := config["mode"].(string); ok {
				prefixSet.Mode = stripNamespace(mode)
			}
		}

		// Get prefixes
		if prefixes, ok := psData["prefixes"].(map[string]interface{}); ok {
			if prefixList, ok := prefixes["prefix"].([]interface{}); ok {
				for _, p := range prefixList {
					pData, ok := p.(map[string]interface{})
					if !ok {
						continue
					}

					prefix := model.Prefix{}

					if ip, ok := pData["ip-prefix"].(string); ok {
						prefix.Prefix = ip
					}

					if maskRange, ok := pData["masklength-range"].(string); ok {
						prefix.MaskLengthRange = maskRange
					}

					// Also check config block
					if config, ok := pData["config"].(map[string]interface{}); ok {
						if ip, ok := config["ip-prefix"].(string); ok {
							prefix.Prefix = ip
						}
						if maskRange, ok := config["masklength-range"].(string); ok {
							prefix.MaskLengthRange = maskRange
						}
					}

					if prefix.Prefix != "" {
						prefixSet.Prefixes = append(prefixSet.Prefixes, prefix)
					}
				}
			}
		}

		if prefixSet.Name != "" {
			e.routingPolicy.DefinedSets.PrefixSets = append(e.routingPolicy.DefinedSets.PrefixSets, prefixSet)
		}
	}
}

func (e *RoutingPolicyExporter) parseBGPDefinedSets(data map[string]interface{}) {
	// Parse community-sets
	if communitySets, ok := data["community-sets"].(map[string]interface{}); ok {
		if setList, ok := communitySets["community-set"].([]interface{}); ok {
			for _, cs := range setList {
				csData, ok := cs.(map[string]interface{})
				if !ok {
					continue
				}

				communitySet := model.CommunitySet{}

				if name, ok := csData["community-set-name"].(string); ok {
					communitySet.Name = name
				}

				if config, ok := csData["config"].(map[string]interface{}); ok {
					if members, ok := config["community-member"].([]interface{}); ok {
						for _, m := range members {
							if member, ok := m.(string); ok {
								communitySet.Members = append(communitySet.Members, member)
							}
						}
					}
				}

				if communitySet.Name != "" {
					e.routingPolicy.DefinedSets.CommunitySets = append(e.routingPolicy.DefinedSets.CommunitySets, communitySet)
				}
			}
		}
	}

	// Parse as-path-sets
	if asPathSets, ok := data["as-path-sets"].(map[string]interface{}); ok {
		if setList, ok := asPathSets["as-path-set"].([]interface{}); ok {
			for _, as := range setList {
				asData, ok := as.(map[string]interface{})
				if !ok {
					continue
				}

				asPathSet := model.ASPathSet{}

				if name, ok := asData["as-path-set-name"].(string); ok {
					asPathSet.Name = name
				}

				if config, ok := asData["config"].(map[string]interface{}); ok {
					if members, ok := config["as-path-set-member"].([]interface{}); ok {
						for _, m := range members {
							if member, ok := m.(string); ok {
								asPathSet.Members = append(asPathSet.Members, member)
							}
						}
					}
				}

				if asPathSet.Name != "" {
					e.routingPolicy.DefinedSets.ASPathSets = append(e.routingPolicy.DefinedSets.ASPathSets, asPathSet)
				}
			}
		}
	}
}

func (e *RoutingPolicyExporter) parsePolicyDefinitions(data map[string]interface{}) {
	var policyList []interface{}

	if policies, ok := data["policy-definition"].([]interface{}); ok {
		policyList = policies
	}

	for _, p := range policyList {
		pData, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		policy := model.PolicyDefinition{}

		if name, ok := pData["name"].(string); ok {
			policy.Name = name
		}

		// Parse statements
		if statements, ok := pData["statements"].(map[string]interface{}); ok {
			if stmtList, ok := statements["statement"].([]interface{}); ok {
				for _, s := range stmtList {
					sData, ok := s.(map[string]interface{})
					if !ok {
						continue
					}

					stmt := model.PolicyStatement{}

					if name, ok := sData["name"].(string); ok {
						stmt.Name = name
					}

					// Parse conditions
					if conditions, ok := sData["conditions"].(map[string]interface{}); ok {
						stmt.Conditions = e.parseConditions(conditions)
					}

					// Parse actions
					if actions, ok := sData["actions"].(map[string]interface{}); ok {
						stmt.Actions = e.parseActions(actions)
					}

					policy.Statements = append(policy.Statements, stmt)
				}
			}
		}

		if policy.Name != "" {
			e.routingPolicy.PolicyDefinitions = append(e.routingPolicy.PolicyDefinitions, policy)
		}
	}
}

func (e *RoutingPolicyExporter) parseConditions(data map[string]interface{}) *model.PolicyConditions {
	conditions := &model.PolicyConditions{}
	hasConditions := false

	// Match prefix set
	if matchPrefixSet, ok := data["match-prefix-set"].(map[string]interface{}); ok {
		if config, ok := matchPrefixSet["config"].(map[string]interface{}); ok {
			if prefixSet, ok := config["prefix-set"].(string); ok {
				conditions.MatchPrefixSet = prefixSet
				hasConditions = true
			}
		}
	}

	// BGP conditions
	if bgpConditions, ok := data["openconfig-bgp-policy:bgp-conditions"].(map[string]interface{}); ok {
		e.parseBGPConditions(conditions, bgpConditions)
		hasConditions = true
	} else if bgpConditions, ok := data["bgp-conditions"].(map[string]interface{}); ok {
		e.parseBGPConditions(conditions, bgpConditions)
		hasConditions = true
	}

	if hasConditions {
		return conditions
	}
	return nil
}

func (e *RoutingPolicyExporter) parseBGPConditions(conditions *model.PolicyConditions, data map[string]interface{}) {
	// Match community set
	if matchCommSet, ok := data["match-community-set"].(map[string]interface{}); ok {
		if config, ok := matchCommSet["config"].(map[string]interface{}); ok {
			if commSet, ok := config["community-set"].(string); ok {
				conditions.MatchCommunitySet = commSet
			}
		}
	}

	// Match AS path set
	if matchASPath, ok := data["match-as-path-set"].(map[string]interface{}); ok {
		if config, ok := matchASPath["config"].(map[string]interface{}); ok {
			if asPathSet, ok := config["as-path-set"].(string); ok {
				conditions.MatchASPathSet = asPathSet
			}
		}
	}

	// Match next-hop
	if config, ok := data["config"].(map[string]interface{}); ok {
		if nextHop, ok := config["next-hop-in"].([]interface{}); ok && len(nextHop) > 0 {
			if nh, ok := nextHop[0].(string); ok {
				conditions.MatchNextHop = nh
			}
		}
	}
}

func (e *RoutingPolicyExporter) parseActions(data map[string]interface{}) *model.PolicyActions {
	actions := &model.PolicyActions{}
	hasActions := false

	// Policy result (accept/reject)
	if config, ok := data["config"].(map[string]interface{}); ok {
		if result, ok := config["policy-result"].(string); ok {
			result = strings.ToUpper(stripNamespace(result))
			if result == "ACCEPT_ROUTE" || result == "ACCEPT" {
				t := true
				actions.Accept = &t
				hasActions = true
			} else if result == "REJECT_ROUTE" || result == "REJECT" {
				t := true
				actions.Reject = &t
				hasActions = true
			}
		}
	}

	// BGP actions
	if bgpActions, ok := data["openconfig-bgp-policy:bgp-actions"].(map[string]interface{}); ok {
		e.parseBGPActions(actions, bgpActions)
		hasActions = true
	} else if bgpActions, ok := data["bgp-actions"].(map[string]interface{}); ok {
		e.parseBGPActions(actions, bgpActions)
		hasActions = true
	}

	if hasActions {
		return actions
	}
	return nil
}

func (e *RoutingPolicyExporter) parseBGPActions(actions *model.PolicyActions, data map[string]interface{}) {
	if config, ok := data["config"].(map[string]interface{}); ok {
		// Set local preference
		if localPref, ok := config["set-local-pref"].(float64); ok && localPref > 0 {
			actions.SetLocalPref = int(localPref)
		}

		// Set MED
		if med, ok := config["set-med"].(float64); ok && med > 0 {
			actions.SetMED = int(med)
		}

		// Set next-hop
		if nextHop, ok := config["set-next-hop"].(string); ok {
			actions.SetNextHop = nextHop
		}
	}

	// Set community
	if setCommunity, ok := data["set-community"].(map[string]interface{}); ok {
		if config, ok := setCommunity["config"].(map[string]interface{}); ok {
			if method, ok := config["method"].(string); ok {
				if strings.Contains(method, "INLINE") {
					if communities, ok := config["communities"].([]interface{}); ok && len(communities) > 0 {
						var commList []string
						for _, c := range communities {
							if comm, ok := c.(string); ok {
								commList = append(commList, comm)
							}
						}
						actions.SetCommunity = strings.Join(commList, " ")
					}
				}
			}
		}
	}

	// AS path prepend
	if setASPath, ok := data["set-as-path-prepend"].(map[string]interface{}); ok {
		if config, ok := setASPath["config"].(map[string]interface{}); ok {
			if repeatN, ok := config["repeat-n"].(float64); ok && repeatN > 0 {
				// OpenConfig uses repeat-n with the local AS
				actions.SetASPathPrepend = "repeat"
			}
			if asn, ok := config["asn"].(float64); ok && asn > 0 {
				actions.SetASPathPrepend = string(rune(int(asn)))
			}
		}
	}
}

func (e *RoutingPolicyExporter) Apply(m *model.DeviceModel) {
	// Only apply if we have meaningful content
	if e.routingPolicy.DefinedSets != nil || len(e.routingPolicy.PolicyDefinitions) > 0 {
		m.RoutingPolicy = e.routingPolicy
	}
}
