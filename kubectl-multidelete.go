package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/spaceweasel/promptui"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/strings/slices"

	// Removes the bell that otherwise rings everytime the line changes.
	_ "github.com/Harwayne/kubectl-select/pkg/removebell"
)

var (
	kubectl = flag.String("kubectl", "kubectl", "kubectl command")
)

func main() {
	flag.Parse()

	var resourceType string
	var selectors []string
	switch len(os.Args) {
	case 1:
		panic(fmt.Errorf("not enough arguments. Usage kubectl multidelete <type> [-l selector]"))
	case 2:
		resourceType = os.Args[1]
	default:
		resourceType = os.Args[1]
		selectors = os.Args[2:]
	}
	if slices.Contains(selectors, "--all-namespaces") {
		fmt.Println("--all-namespaces is not supported")
		return
	}
	objects := listObjects(resourceType, selectors)
	if len(objects) == 0 {
		fmt.Println("No objects found")
		return
	}
	selectedObjects, err := displayAndChooseObjects(resourceType, objects)
	if err != nil {
		fmt.Printf("Selecting Objects: %v\n", err)
		return
	}
	err = deletedObjects(resourceType, selectedObjects)
	if err != nil {
		fmt.Printf("Deleting objects: %v\n", err)
		return
	}
}

type kubernetesObjectList struct {
	Items []kubernetesObject `json:"items"`
}

type kubernetesObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

func listObjects(resourceType string, selectors []string) []kubernetesObject {
	commandArgs := []string{"get", "-ojson"}
	commandArgs = append(commandArgs, resourceType)
	commandArgs = append(commandArgs, selectors...)
	cmd := exec.Command(*kubectl, commandArgs...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Errorf("listing configurations: %q, %w", cmd, err))
	}
	var list kubernetesObjectList
	if err := json.Unmarshal(b, &list); err != nil {
		panic(fmt.Errorf("json unmarshalling bytes: %q, %w", string(b), err))
	}
	return list.Items
}

func displayAndChooseObjects(resourceType string, objects []kubernetesObject) ([]kubernetesObject, error) {
	prompt := promptui.MultiSelect{
		Label: fmt.Sprintf("Select resources of type %s to delete", resourceType),
		Items: objects,
		Size:  smaller(60, len(objects)),
		Templates: &promptui.MultiSelectTemplates{
			Selected:   "Delete - {{ .Name }}",
			Unselected: "Keep   - {{ .Name }}",
		},
	}
	indices, err := prompt.Run()
	if err != nil {
		return []kubernetesObject{}, fmt.Errorf("Prompt failed: %v\n", err)
	}
	var selectedObjects []kubernetesObject
	for _, selectedIndex := range indices {
		selectedObjects = append(selectedObjects, objects[selectedIndex])
	}
	return selectedObjects, nil
}

func smaller(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func deletedObjects(resourceType string, objects []kubernetesObject) error {
	if len(objects) == 0 {
		fmt.Println("Nothing to delete")
		return nil
	}
	args := []string{"delete", resourceType}
	args = append(args, kubectlNamespaceFlag(objects[0])...)
	for _, o := range objects {
		args = append(args, o.Name)
	}
	cmd := exec.Command(*kubectl, args...)
	fmt.Printf("Running: %q\n", cmd)
	b, err := cmd.CombinedOutput()
	fmt.Print(string(b))
	if err != nil {
		return fmt.Errorf("deleting objects: %w", err)
	}
	return nil
}

func kubectlNamespaceFlag(o kubernetesObject) []string {
	if o.Namespace != "" {
		return []string{"-n", o.Namespace}
	}
	return []string{}
}
