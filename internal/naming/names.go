package naming

import "strings"

// KubernetesPart normalizes a resource identifier for generated Kubernetes names
func KubernetesPart(name string) string {
	var output strings.Builder
	for index, character := range name {
		if index > 0 && character >= 'A' && character <= 'Z' {
			output.WriteByte('-')
		}
		output.WriteRune(character)
	}
	return strings.ToLower(output.String())
}
