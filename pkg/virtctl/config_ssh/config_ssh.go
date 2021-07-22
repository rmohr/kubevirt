/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2019 Red Hat, Inc.
 *
 */

package config_ssh

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"

	k6sv1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt/pkg/virtctl/templates"

	"github.com/kevinburke/ssh_config"
)

const (
	CONFIG_SSH_COMMAND = "config-ssh"
	KubeVirtEOLComment = "Generated by KubeVirt"
)

var (
	dryRun        bool
	removeEntries bool
	sshConfigFile string
	allNamespaces bool
)

func NewConfigSSHCommand(clientConfig clientcmd.ClientConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config-ssh",
		Short: "Write VM(I)s into ~/.ssh/config.",
		Long: `Write VM(I)s into .ssh/config to get convenient aliases in ~/.ssh/config for quickly accessing VMs with ssh.
On most system this will give auto-completion for ssh invocations.
`,
		Example: examples(),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := VirtCommand{
				command:      CONFIG_SSH_COMMAND,
				clientConfig: clientConfig,
			}
			return c.Run(args)
		},
	}
	cmd.SetUsageTemplate(templates.UsageTemplate())
	cmd.Flags().BoolVar(&allNamespaces, "all-namespaces", false, "If present, selects the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show ssh config changes instead of writing them to the file.")
	cmd.Flags().BoolVar(&removeEntries, "remove", false, "Remove all host entries from the ssh config file added by KubeVirt.")
	cmd.Flags().StringVar(&sshConfigFile, "ssh-config-file", "", "Specifies an alternative per-user SSH configuration file. By default, this is ~/.ssh/config.")
	return cmd
}

type VirtCommand struct {
	clientConfig clientcmd.ClientConfig
	command      string
}

func (vc *VirtCommand) Run(args []string) error {

	programName := templates.GetProgramName(filepath.Base(os.Args[0]))

	if sshConfigFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not determine home directory of the user: %v", err)
		}
		sshConfigFile = filepath.Join(home, ".ssh", "config")
	}
	cfg, fileMode, err := loadSSHConfig(sshConfigFile)
	if err != nil {
		return err
	}

	if removeEntries {
		cfg.Hosts = removeHostEntries(cfg.Hosts)
	} else if !removeEntries {
		namespace, _, err := vc.clientConfig.Namespace()
		if err != nil {
			return err
		}

		if allNamespaces {
			namespace = ""
		}

		virtClient, err := kubecli.GetKubevirtClientFromClientConfig(vc.clientConfig)
		if err != nil {
			return fmt.Errorf("cannot obtain KubeVirt client: %v", err)
		}
		rawConfig, err := vc.clientConfig.RawConfig()
		if err != nil {
			return fmt.Errorf("failed to determine current context: %v", err)
		}
		currentContext := rawConfig.CurrentContext

		cfg.Hosts = removeHostEntriesForRegenerate(cfg.Hosts, namespace, currentContext)

		for _, gvr := range []schema.GroupVersionResource{k6sv1.VirtualMachineInstanceGroupVersionResource, k6sv1.VirtualMachineGroupVersionResource} {
			objects, err := virtClient.DynamicClient().Resource(gvr).Namespace(namespace).List(context.Background(), v1.ListOptions{})
			if err != nil {
				return fmt.Errorf("failed to fetch %s: %v", gvr.Resource, err)
			}

			hosts, err := generateHostEntries(programName, currentContext, objects.Items)
			if err != nil {
				return err
			}
			cfg.Hosts = append(cfg.Hosts, hosts...)
		}
	}

	if dryRun {
		fmt.Println(cfg.String())
		return nil
	} else {
		return ioutil.WriteFile(sshConfigFile, []byte(cfg.String()), fileMode)
	}
}

func removeHostEntries(hosts []*ssh_config.Host) (cleanedList []*ssh_config.Host) {
	for i, host := range hosts {
		if host.EOLComment != KubeVirtEOLComment {
			cleanedList = append(cleanedList, hosts[i])
		}
	}
	return cleanedList
}

func removeHostEntriesForRegenerate(hosts []*ssh_config.Host, namespace string, context string) (cleanedList []*ssh_config.Host) {
	for i, host := range hosts {
		if toClean := matchByNamespaceAndContext(host, namespace, context); !toClean {
			cleanedList = append(cleanedList, hosts[i])
		}
	}
	return cleanedList
}

func matchByNamespaceAndContext(host *ssh_config.Host, namespace string, context string) bool {
	if host.EOLComment != KubeVirtEOLComment {
		return false
	}
	split := strings.SplitN(host.Patterns[0].String(), ".", 3)
	context = strings.ReplaceAll(context, "@", "_")
	if len(split) != 3 {
		return false
	}
	if split[2] != context {
		return false
	}

	if namespace != "" && split[1] != namespace {
		return false
	}
	return true
}

func generateHostEntries(programName string, currentContext string, objects []unstructured.Unstructured) (hosts []*ssh_config.Host, err error) {
	for _, obj := range objects {
		host, err := generateHostEntry(programName, currentContext, &obj)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	return hosts, nil
}

func generateHostEntry(programName string, currentContext string, obj *unstructured.Unstructured) (*ssh_config.Host, error) {
	resource := ""
	if obj.GetKind() == k6sv1.VirtualMachineInstanceGroupVersionKind.Kind {
		resource = "vmi"
	} else if obj.GetKind() == k6sv1.VirtualMachineGroupVersionKind.Kind {
		resource = "vm"
	} else {
		return nil, fmt.Errorf("unsupported object kind: %s", obj.GetKind())
	}
	name := fmt.Sprintf("%s/%s.%s.%s", resource, obj.GetName(), obj.GetNamespace(), strings.ReplaceAll(currentContext, "@", "_"))
	pattern, err := ssh_config.NewPattern(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create host entry for %v: %v", name, err)
	}
	host := &ssh_config.Host{
		EOLComment: KubeVirtEOLComment,
		Patterns:   []*ssh_config.Pattern{pattern},
		Nodes: []ssh_config.Node{
			&ssh_config.KV{
				Key:   "ProxyCommand",
				Value: fmt.Sprintf("%s port-forward --context %s --stdio %s/%s.%s %s", programName, currentContext, resource, obj.GetName(), obj.GetNamespace(), "%p"),
			},
		},
	}
	return host, nil
}

func loadSSHConfig(path string) (*ssh_config.Config, os.FileMode, error) {
	var fileMode os.FileMode = 0600
	fileInfo, err := os.Stat(path)
	if os.IsNotExist(err) {
		return &ssh_config.Config{}, fileMode, nil
	} else if err != nil {
		return nil, fileMode, fmt.Errorf("failed to determine if %s exists: %v", path, err)
	}
	fileMode = fileInfo.Mode()

	config, err := os.Open(path)
	if err != nil {
		return nil, fileMode, fmt.Errorf("could not open %s: %v", path, err)
	}
	defer config.Close()
	cfg, err := ssh_config.Decode(config)
	if err != nil {
		return nil, fileMode, fmt.Errorf("failed to decode %s: %v", path, err)
	}
	return cfg, fileMode, nil
}

func examples() string {
	return `  # Add ssh entries for all VMs and VMIs existing in the current namespace and context:
  {{ProgramName}} config-ssh

  # Add ssh entries for all VMs and VMIs from all namespaces in the current context:
  {{ProgramName}} config-ssh --all-namespaces

  # Add ssh entries for all VMs and VMIs from all namespaces in a non-default context:
  {{ProgramName}} config-ssh --all-namespaces --context=othercontext

  # VMIs can be accessed via vmi/NAME.NAMESPACE.CONTEXT
  ssh vmi/<name>.<namespace>.<context>

  # VMs can be accessed via vm/NAME.NAMESPACE.CONTEXT
  ssh vm/<name>.<namespace>.<context>

  # Remove all kubevirt related entries from ~/.ssh/config
  {{ProgramName}} config-ssh --remove
`
}
