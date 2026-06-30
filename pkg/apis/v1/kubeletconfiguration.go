/*
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

package v1

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/samber/lo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ParsedKubeletConfig holds the extracted kubelet configuration fields that Karpenter
// uses for scheduling decisions and bootstrap scripting.
type ParsedKubeletConfig struct {
	ClusterDNS                 []string
	MaxPods                    *int32
	PodsPerCore                *int32
	SystemReserved             map[string]string
	KubeReserved               map[string]string
	EvictionHard               map[string]string
	EvictionSoft               map[string]string
	EvictionSoftGracePeriod    map[string]metav1.Duration
	EvictionMaxPodGracePeriod  *int32
	ImageGCHighThresholdPercent *int32
	ImageGCLowThresholdPercent  *int32
	CPUCFSQuota                *bool
}

// ParseKubeletConfig extracts known fields from the unstructured kubelet config map
// into a typed struct for use in scheduling and bootstrap logic.
func ParseKubeletConfig(kc KubeletConfiguration) (*ParsedKubeletConfig, error) {
	if len(kc) == 0 {
		return &ParsedKubeletConfig{}, nil
	}
	parsed := &ParsedKubeletConfig{}

	if v, ok := kc["clusterDNS"]; ok {
		if err := unmarshalJSON(v, &parsed.ClusterDNS); err != nil {
			return nil, fieldError("clusterDNS", err)
		}
	}
	if v, ok := kc["maxPods"]; ok {
		val, err := unmarshalInt32(v)
		if err != nil {
			return nil, fieldError("maxPods", err)
		}
		parsed.MaxPods = val
	}
	if v, ok := kc["podsPerCore"]; ok {
		val, err := unmarshalInt32(v)
		if err != nil {
			return nil, fieldError("podsPerCore", err)
		}
		parsed.PodsPerCore = val
	}
	if v, ok := kc["systemReserved"]; ok {
		if err := unmarshalJSON(v, &parsed.SystemReserved); err != nil {
			return nil, fieldError("systemReserved", err)
		}
	}
	if v, ok := kc["kubeReserved"]; ok {
		if err := unmarshalJSON(v, &parsed.KubeReserved); err != nil {
			return nil, fieldError("kubeReserved", err)
		}
	}
	if v, ok := kc["evictionHard"]; ok {
		if err := unmarshalJSON(v, &parsed.EvictionHard); err != nil {
			return nil, fieldError("evictionHard", err)
		}
	}
	if v, ok := kc["evictionSoft"]; ok {
		if err := unmarshalJSON(v, &parsed.EvictionSoft); err != nil {
			return nil, fieldError("evictionSoft", err)
		}
	}
	if v, ok := kc["evictionSoftGracePeriod"]; ok {
		raw := map[string]string{}
		if err := unmarshalJSON(v, &raw); err != nil {
			return nil, fieldError("evictionSoftGracePeriod", err)
		}
		parsed.EvictionSoftGracePeriod = make(map[string]metav1.Duration, len(raw))
		for k, durStr := range raw {
			d, err := time.ParseDuration(durStr)
			if err != nil {
				return nil, fieldError("evictionSoftGracePeriod", fmt.Errorf("invalid duration for key %q: %w", k, err))
			}
			parsed.EvictionSoftGracePeriod[k] = metav1.Duration{Duration: d}
		}
	}
	if v, ok := kc["evictionMaxPodGracePeriod"]; ok {
		val, err := unmarshalInt32(v)
		if err != nil {
			return nil, fieldError("evictionMaxPodGracePeriod", err)
		}
		parsed.EvictionMaxPodGracePeriod = val
	}
	if v, ok := kc["imageGCHighThresholdPercent"]; ok {
		val, err := unmarshalInt32(v)
		if err != nil {
			return nil, fieldError("imageGCHighThresholdPercent", err)
		}
		parsed.ImageGCHighThresholdPercent = val
	}
	if v, ok := kc["imageGCLowThresholdPercent"]; ok {
		val, err := unmarshalInt32(v)
		if err != nil {
			return nil, fieldError("imageGCLowThresholdPercent", err)
		}
		parsed.ImageGCLowThresholdPercent = val
	}
	if v, ok := kc["cpuCFSQuota"]; ok {
		var b bool
		if err := unmarshalJSON(v, &b); err != nil {
			return nil, fieldError("cpuCFSQuota", err)
		}
		parsed.CPUCFSQuota = lo.ToPtr(b)
	}
	return parsed, nil
}

// DeepCopy returns a deep copy of ParsedKubeletConfig.
func (in *ParsedKubeletConfig) DeepCopy() *ParsedKubeletConfig {
	if in == nil {
		return nil
	}
	out := &ParsedKubeletConfig{}
	if in.ClusterDNS != nil {
		out.ClusterDNS = make([]string, len(in.ClusterDNS))
		copy(out.ClusterDNS, in.ClusterDNS)
	}
	out.MaxPods = copyInt32Ptr(in.MaxPods)
	out.PodsPerCore = copyInt32Ptr(in.PodsPerCore)
	out.SystemReserved = copyStringMap(in.SystemReserved)
	out.KubeReserved = copyStringMap(in.KubeReserved)
	out.EvictionHard = copyStringMap(in.EvictionHard)
	out.EvictionSoft = copyStringMap(in.EvictionSoft)
	if in.EvictionSoftGracePeriod != nil {
		out.EvictionSoftGracePeriod = make(map[string]metav1.Duration, len(in.EvictionSoftGracePeriod))
		for k, v := range in.EvictionSoftGracePeriod {
			out.EvictionSoftGracePeriod[k] = v
		}
	}
	out.EvictionMaxPodGracePeriod = copyInt32Ptr(in.EvictionMaxPodGracePeriod)
	out.ImageGCHighThresholdPercent = copyInt32Ptr(in.ImageGCHighThresholdPercent)
	out.ImageGCLowThresholdPercent = copyInt32Ptr(in.ImageGCLowThresholdPercent)
	if in.CPUCFSQuota != nil {
		out.CPUCFSQuota = lo.ToPtr(*in.CPUCFSQuota)
	}
	return out
}

func unmarshalJSON(v apiextensionsv1.JSON, target interface{}) error {
	return json.Unmarshal(v.Raw, target)
}

func unmarshalInt32(v apiextensionsv1.JSON) (*int32, error) {
	var f float64
	if err := json.Unmarshal(v.Raw, &f); err != nil {
		return nil, fmt.Errorf("must be a number: %w", err)
	}
	val := int32(f)
	return &val, nil
}

func fieldError(field string, err error) error {
	return fmt.Errorf("spec.kubelet.%s: %w", field, err)
}

func copyInt32Ptr(p *int32) *int32 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
