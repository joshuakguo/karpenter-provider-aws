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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// MustMakeKubeletConfiguration constructs a KubeletConfiguration map from an arbitrary
// struct or map by marshaling to JSON and back. This is intended for use in tests and
// places where constructing the map from typed fields is more ergonomic.
func MustMakeKubeletConfiguration(obj interface{}) KubeletConfiguration {
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	kc := KubeletConfiguration{}
	if err := json.Unmarshal(data, &kc); err != nil {
		panic(err)
	}
	return kc
}

// JSONValue is a helper to create an apiextensionsv1.JSON from any value.
func JSONValue(v interface{}) apiextensionsv1.JSON {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return apiextensionsv1.JSON{Raw: raw}
}
