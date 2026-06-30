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
	"net"
	"reflect"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
)

// knownKubeletFields is derived at init time from the upstream KubeletConfiguration struct
// so it stays in sync with the Kubernetes version Karpenter depends on.
var knownKubeletFields = initKnownKubeletFields()

func initKnownKubeletFields() sets.Set[string] {
	s := sets.New[string]()
	t := reflect.TypeOf(kubeletconfigv1beta1.KubeletConfiguration{})
	for i := range t.NumField() {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		name := strings.Split(jsonTag, ",")[0]
		if name != "" {
			s.Insert(name)
		}
	}
	return s
}

var validEvictionSignals = sets.New(
	"memory.available",
	"nodefs.available",
	"nodefs.inodesFree",
	"imagefs.available",
	"imagefs.inodesFree",
	"pid.available",
)

var validReservedResourceKeys = sets.New(
	"cpu",
	"memory",
	"ephemeral-storage",
	"pid",
)

// ValidateKubeletConfig validates the unstructured kubelet configuration map.
// It checks field names against the upstream kubelet schema (derived via reflection)
// and validates known fields for correct types and values.
func ValidateKubeletConfig(kc KubeletConfiguration) []error {
	if len(kc) == 0 {
		return nil
	}

	var errs []error

	// Validate all field names are known upstream kubelet fields
	for key := range kc {
		if !knownKubeletFields.Has(key) {
			errs = append(errs, fmt.Errorf("spec.kubelet.%s: unrecognized kubelet configuration field", key))
		}
	}

	// Parse and validate scheduling-relevant and known passthrough fields
	parsed, err := ParseKubeletConfig(kc)
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	if parsed.MaxPods != nil && *parsed.MaxPods < 0 {
		errs = append(errs, fmt.Errorf("spec.kubelet.maxPods: must be a non-negative integer"))
	}
	if parsed.PodsPerCore != nil && *parsed.PodsPerCore < 0 {
		errs = append(errs, fmt.Errorf("spec.kubelet.podsPerCore: must be a non-negative integer"))
	}

	errs = append(errs, validateReservedResources("kubeReserved", parsed.KubeReserved)...)
	errs = append(errs, validateReservedResources("systemReserved", parsed.SystemReserved)...)
	errs = append(errs, validateEvictionMap("evictionHard", parsed.EvictionHard)...)
	errs = append(errs, validateEvictionMap("evictionSoft", parsed.EvictionSoft)...)

	// Validate evictionSoftGracePeriod keys and durations
	if parsed.EvictionSoftGracePeriod != nil {
		for key := range parsed.EvictionSoftGracePeriod {
			if !validEvictionSignals.Has(key) {
				errs = append(errs, fmt.Errorf("spec.kubelet.evictionSoftGracePeriod: invalid eviction signal %q", key))
			}
		}
	}

	// Validate evictionSoft and evictionSoftGracePeriod are matched
	if parsed.EvictionSoft != nil {
		for key := range parsed.EvictionSoft {
			if parsed.EvictionSoftGracePeriod == nil {
				errs = append(errs, fmt.Errorf("spec.kubelet.evictionSoft: key %q does not have a matching evictionSoftGracePeriod", key))
				break
			}
			if _, ok := parsed.EvictionSoftGracePeriod[key]; !ok {
				errs = append(errs, fmt.Errorf("spec.kubelet.evictionSoft: key %q does not have a matching evictionSoftGracePeriod", key))
			}
		}
	}
	if parsed.EvictionSoftGracePeriod != nil {
		for key := range parsed.EvictionSoftGracePeriod {
			if parsed.EvictionSoft == nil {
				errs = append(errs, fmt.Errorf("spec.kubelet.evictionSoftGracePeriod: key %q does not have a matching evictionSoft", key))
				break
			}
			if _, ok := parsed.EvictionSoft[key]; !ok {
				errs = append(errs, fmt.Errorf("spec.kubelet.evictionSoftGracePeriod: key %q does not have a matching evictionSoft", key))
			}
		}
	}

	if parsed.EvictionMaxPodGracePeriod != nil && *parsed.EvictionMaxPodGracePeriod < 0 {
		errs = append(errs, fmt.Errorf("spec.kubelet.evictionMaxPodGracePeriod: must be a non-negative integer"))
	}

	// Validate imageGC thresholds
	if parsed.ImageGCHighThresholdPercent != nil {
		if *parsed.ImageGCHighThresholdPercent < 0 || *parsed.ImageGCHighThresholdPercent > 100 {
			errs = append(errs, fmt.Errorf("spec.kubelet.imageGCHighThresholdPercent: must be between 0 and 100"))
		}
	}
	if parsed.ImageGCLowThresholdPercent != nil {
		if *parsed.ImageGCLowThresholdPercent < 0 || *parsed.ImageGCLowThresholdPercent > 100 {
			errs = append(errs, fmt.Errorf("spec.kubelet.imageGCLowThresholdPercent: must be between 0 and 100"))
		}
	}
	if parsed.ImageGCHighThresholdPercent != nil && parsed.ImageGCLowThresholdPercent != nil {
		if *parsed.ImageGCHighThresholdPercent <= *parsed.ImageGCLowThresholdPercent {
			errs = append(errs, fmt.Errorf("spec.kubelet.imageGCHighThresholdPercent: must be greater than imageGCLowThresholdPercent"))
		}
	}

	// Validate clusterDNS
	for _, ip := range parsed.ClusterDNS {
		if net.ParseIP(ip) == nil {
			errs = append(errs, fmt.Errorf("spec.kubelet.clusterDNS: %q is not a valid IP address", ip))
		}
	}

	// Validate evictionSoftGracePeriod duration values from raw input
	if v, ok := kc["evictionSoftGracePeriod"]; ok {
		raw := map[string]string{}
		if unmarshalErr := json.Unmarshal(v.Raw, &raw); unmarshalErr == nil {
			for key, durStr := range raw {
				if _, parseErr := time.ParseDuration(durStr); parseErr != nil {
					errs = append(errs, fmt.Errorf("spec.kubelet.evictionSoftGracePeriod: invalid duration %q for key %q", durStr, key))
				}
			}
		}
	}

	return errs
}

func validateReservedResources(field string, m map[string]string) []error {
	var errs []error
	for key, val := range m {
		if !validReservedResourceKeys.Has(key) {
			errs = append(errs, fmt.Errorf("spec.kubelet.%s: invalid key %q, valid keys are [cpu, memory, ephemeral-storage, pid]", field, key))
		}
		if _, err := resource.ParseQuantity(val); err != nil {
			errs = append(errs, fmt.Errorf("spec.kubelet.%s: invalid resource quantity %q for key %q", field, val, key))
		}
		if strings.HasPrefix(val, "-") {
			errs = append(errs, fmt.Errorf("spec.kubelet.%s: value for key %q cannot be negative", field, key))
		}
	}
	return errs
}

func validateEvictionMap(field string, m map[string]string) []error {
	var errs []error
	for key := range m {
		if !validEvictionSignals.Has(key) {
			errs = append(errs, fmt.Errorf("spec.kubelet.%s: invalid eviction signal %q", field, key))
		}
	}
	return errs
}
