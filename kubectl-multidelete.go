package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spaceweasel/promptui"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/utils/strings/slices"

	// Removes the bell that otherwise rings everytime the line changes.
	_ "github.com/Harwayne/kubectl-select/pkg/removebell"
)

var (
	kubectl      = flag.String("kubectl", "kubectl", "kubectl command")
	preselectAll = flag.Bool("preselect-all", false, "Preselect everything for deletion")
)

func main() {
	flag.Parse()

	if len(flag.Args()) == 0 {
		fmt.Println("Usage:  kubectl multidelete [-preselect-all] <type> [other kubectl get flags]")
		fmt.Println("Example: kubectl multidelete ns")
		fmt.Println("Example: kubectl multidelete -preselect-all po -n foo -l bar=qux")
		os.Exit(1)
	}

	resourceType := flag.Arg(0)
	selectors := flag.Args()[1:]

	if slices.Contains(selectors, "--all-namespaces") || slices.Contains(selectors, "-A") {
		fmt.Println("-A, --all-namespaces is not supported")
		os.Exit(1)
	}
	ns := extractNamespace(selectors)

	objects := listObjects(resourceType, selectors)
	if len(objects) == 0 {
		fmt.Println("No objects found")
		os.Exit(1)
	}
	selectedObjects, err := displayAndChooseObjects(resourceType, objects)
	if err != nil {
		fmt.Printf("Selecting Objects: %v\n", err)
		os.Exit(1)
	}

	err = deleteObjects(ns, resourceType, selectedObjects)
	if err != nil {
		fmt.Printf("Deleting objects: %v\n", err)
		os.Exit(1)
	}
}

func extractNamespace(selectors []string) string {
	for i := 0; i < len(selectors); i++ {
		s := selectors[i]
		if p := "-n="; strings.HasPrefix(s, p) {
			return s[len(p):]
		}
		if p := "--namespace="; strings.HasPrefix(s, p) {
			return s[len(p):]
		}
		if s == "-n" || s == "--namespace" {
			if j := i + 1; j < len(selectors) {
				return selectors[j]
			}
			panic("Cannot extract namespace, last argument is -n/--namespace.")
		}
	}
	return ""
}

func listObjects(resourceType string, selectors []string) []string {
	commandArgs := []string{"get", "--no-headers"}
	commandArgs = append(commandArgs, resourceType)
	commandArgs = append(commandArgs, selectors...)
	cmd := exec.Command(*kubectl, commandArgs...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Errorf("listing configurations: %q, %w\n%s", cmd, err, string(b)))
	}
	s := strings.TrimSpace(string(b))
	if strings.HasPrefix(s, "No resources found") {
		return []string{}
	}
	list := strings.Split(s, "\n")
	return list
}

func displayAndChooseObjects(resourceType string, objects []string) ([]string, error) {
	_, terminalHeight, err := terminal.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// Default to 24 for no particular reason.
		terminalHeight = 24
	}
	var preselected []int
	if *preselectAll {
		preselected = make([]int, len(objects))
		for i := 0; i < len(preselected); i++ {
			preselected[i] = i
		}
	}
	prompt := promptui.MultiSelect{
		Label: fmt.Sprintf("Select resources of type %s to delete", resourceType),
		Items: objects,
		Size:  smaller(len(objects), terminalHeight-3),
		Templates: &promptui.MultiSelectTemplates{
			Active:     fmt.Sprintf("%s{{ . | underline }}", promptui.IconSelect),
			Selected:   "Delete - {{ . }}",
			Unselected: "Keep   - {{ . }}",
		},
		Selected: preselected,
	}
	indices, err := prompt.Run()
	if err != nil {
		return []string{}, fmt.Errorf("Prompt failed: %v\n", err)
	}
	var selectedObjects []string
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

func deleteObjects(ns, resourceType string, objects []string) error {
	if len(objects) == 0 {
		fmt.Println("Nothing to delete")
		return nil
	}
	args := []string{"delete"}
	if ns != "" {
		args = append(args, "-n", ns)
	}
	if resourceType != "all" {
		args = append(args, resourceType)
	}
	for _, o := range objects {
		args = append(args, extractNameFromKubectlLine(o))
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

func extractNameFromKubectlLine(o string) string {
	// Assume it is the first column.
	return strings.SplitN(o, " ", 2)[0]
}
