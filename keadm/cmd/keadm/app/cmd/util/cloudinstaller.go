package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	types "github.com/kubeedge/kubeedge/keadm/cmd/keadm/app/cmd/common"
	"github.com/kubeedge/kubeedge/pkg/apis/componentconfig/cloudcore/v1alpha1"
	"github.com/kubeedge/kubeedge/pkg/version"
	crdclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//KubeCloudInstTool embedes Common struct
//It implements ToolsInstaller interface
type KubeCloudInstTool struct {
	Common
	AdvertiseAddress string
}

// InstallTools downloads KubeEdge for the specified version
// and makes the required configuration changes and initiates cloudcore.
func (cu *KubeCloudInstTool) InstallTools() error {
	cu.SetOSInterface(GetOSInterface())
	cu.SetKubeEdgeVersion(cu.ToolVersion)

	if cu.ToolVersion < "1.3.0" {
		if err := cu.InstallKubeEdge(types.CloudCore); err != nil {
			return err
		}

		if err := cu.generateCertificates(); err != nil {
			return err
		}

		if err := cu.tarCertificates(); err != nil {
			return err
		}
	}

	if cu.ToolVersion >= "1.2.0" {
		//This makes sure the path is created, if it already exists also it is fine
		err := os.MkdirAll(KubeEdgeNewConfigDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("not able to create %s folder path", KubeEdgeNewConfigDir)
		}

		cloudCoreConfig := v1alpha1.NewDefaultCloudCoreConfig()
		if cu.KubeConfig != "" {
			cloudCoreConfig.KubeAPIConfig.KubeConfig = cu.KubeConfig
		}

		if cu.Master != "" {
			cloudCoreConfig.KubeAPIConfig.Master = cu.Master
		}

		if cu.AdvertiseAddress != "" {
			cloudCoreConfig.Modules.CloudHub.AdvertiseAddress = strings.Split(cu.AdvertiseAddress, ",")
		}

		if strings.HasPrefix(cu.ToolVersion, "1.2") {
			cloudCoreConfig.Modules.CloudHub.TLSPrivateKeyFile = KubeEdgeCloudDefaultCertPath + "server.key"
			cloudCoreConfig.Modules.CloudHub.TLSCertFile = KubeEdgeCloudDefaultCertPath + "server.crt"
		}
		if err := types.Write2File(KubeEdgeCloudCoreNewYaml, cloudCoreConfig); err != nil {
			return err
		}
	} else {
		//This makes sure the path is created, if it already exists also it is fine
		err := os.MkdirAll(KubeEdgeCloudConfPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("not able to create %s folder path", KubeEdgeConfPath)
		}

		//KubeEdgeCloudCoreYaml:= fmt.Sprintf("%s%s/edge/%s",KubeEdgePath)
		//	KubeEdgePath, KubeEdgePath, filename, KubeEdgePath, dirname, KubeEdgeBinaryName, KubeEdgeUsrBinPath)
		//Create controller.yaml
		if err = types.WriteControllerYamlFile(KubeEdgeCloudCoreYaml, cu.KubeConfig); err != nil {
			return err
		}

		//Create modules.yaml
		if err = types.WriteCloudModulesYamlFile(KubeEdgeCloudCoreModulesYaml); err != nil {
			return err
		}
	}

	time.Sleep(1 * time.Second)

	err := cu.RunCloudCore()
	if err != nil {
		return err
	}
	fmt.Println("CloudCore started")

	return nil
}

//generateCertificates - Certifcates ca,cert will be generated in /etc/kubeedge/
func (cu *KubeCloudInstTool) generateCertificates() error {
	//Create certgen.sh
	if err := ioutil.WriteFile(KubeEdgeCloudCertGenPath, CertGenSh, 0775); err != nil {
		return err
	}

	cmd := &Command{Cmd: exec.Command("bash", "-x", KubeEdgeCloudCertGenPath, "genCertAndKey", "server")}
	err := cmd.ExecuteCmdShowOutput()
	stdout := cmd.GetStdOutput()
	errout := cmd.GetStdErr()
	if err != nil || errout != "" {
		return fmt.Errorf("%s", "certificates not installed")
	}
	fmt.Println(stdout)
	fmt.Println("Certificates got generated at:", KubeEdgePath, "ca and", KubeEdgePath, "certs")
	return nil
}

//tarCertificates - certs will be tared at /etc/kubeedge/kubeedge/certificates/certs
func (cu *KubeCloudInstTool) tarCertificates() error {
	tarCmd := fmt.Sprintf("tar -cvzf %s %s", KubeEdgeEdgeCertsTarFileName, strings.Split(KubeEdgeEdgeCertsTarFileName, ".")[0])
	cmd := &Command{Cmd: exec.Command("sh", "-c", tarCmd)}
	cmd.Cmd.Dir = KubeEdgePath
	err := cmd.ExecuteCmdShowOutput()
	stdout := cmd.GetStdOutput()
	errout := cmd.GetStdErr()
	if err != nil || errout != "" {
		return fmt.Errorf("%s", "error in tarring the certificates")
	}
	fmt.Println(stdout)
	fmt.Println("Certificates got tared at:", KubeEdgePath, "path, Please copy it to desired edge node (at", KubeEdgePath, "path)")
	return nil
}

//RunCloudCore starts cloudcore process
func (cu *KubeCloudInstTool) RunCloudCore() error {
	// above 1.3.0, cloudcore run as deployment which is installed in k8sinstaller.go
	if cu.ToolVersion >= "1.3.0" {
		return nil
	}

	// create the log dir for kubeedge
	err := os.MkdirAll(KubeEdgeLogPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("not able to create %s folder path", KubeEdgeLogPath)
	}

	// add +x for cloudcore
	command := fmt.Sprintf("chmod +x %s/%s", KubeEdgeUsrBinPath, KubeCloudBinaryName)
	if _, err := runCommandWithShell(command); err != nil {
		return err
	}

	// start cloudcore
	if cu.ToolVersion >= "1.1.0" {
		command = fmt.Sprintf(" %s > %s/%s.log 2>&1 &", KubeCloudBinaryName, KubeEdgeLogPath, KubeCloudBinaryName)
	} else {
		command = fmt.Sprintf("%s > %skubeedge/cloud/%s.log 2>&1 &", KubeCloudBinaryName, KubeEdgePath, KubeCloudBinaryName)
	}
	cmd := &Command{Cmd: exec.Command("sh", "-c", command)}
	cmd.Cmd.Env = os.Environ()
	env := fmt.Sprintf("GOARCHAIUS_CONFIG_PATH=%skubeedge/cloud", KubeEdgePath)
	cmd.Cmd.Env = append(cmd.Cmd.Env, env)
	cmd.ExecuteCommand()
	if errout := cmd.GetStdErr(); errout != "" {
		return fmt.Errorf("%s", errout)
	}
	fmt.Println(cmd.GetStdOutput())

	if cu.ToolVersion >= "1.1.0" {
		fmt.Println("KubeEdge cloudcore is running, For logs visit: ", KubeEdgeLogPath+KubeCloudBinaryName+".log")
	} else {
		fmt.Println("KubeEdge cloudcore is running, For logs visit", KubeEdgePath+"kubeedge/cloud/")
	}

	return nil
}

//TearDown method will remove the edge node from api-server and stop cloudcore process
func (cu *KubeCloudInstTool) TearDown() error {
	cu.SetOSInterface(GetOSInterface())

	if version.Get().GitVersion >= "v1.3.0" {
		config, err := BuildConfig(cu.KubeConfig, cu.Master)
		if err != nil {
			return fmt.Errorf("failed to build config, err: %v", err)
		}

		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return fmt.Errorf("failed to create client, err: %v", err)
		}

		crdClient, err := crdclient.NewForConfig(config)
		if err != nil {
			return err
		}

		deletePolicy := metav1.DeletePropagationForeground
		deleteOptions := metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}
		if err = client.CoreV1().Namespaces().Delete(KubeEdgeCloudNameSpace, &deleteOptions); err != nil {
			fmt.Printf("failed to delete namespace: %v", err)
		}

		if err = client.RbacV1().ClusterRoles().Delete(KubeCloudBinaryName, &deleteOptions); err != nil {
			fmt.Printf("failed to delete clusterrole: %v", err)
		}

		if err = client.RbacV1().ClusterRoleBindings().Delete(KubeCloudBinaryName, &deleteOptions); err != nil {
			fmt.Printf("failed to delete cloudrolebinding: %v", err)
		}

		for _, name := range []string{
			"devices.devices.kubeedge.io",
			"devicemodels.devices.kubeedge.io",
			"clusterobjectsyncs.reliablesyncs.kubeedge.io",
			"objectsyncs.reliablesyncs.kubeedge.io",
		} {
			if err = crdClient.ApiextensionsV1beta1().CustomResourceDefinitions().Delete(name, &deleteOptions); err != nil {
				fmt.Printf("failed to delete crd: %v", err)
			}
		}
	} else {
		if err := cu.KillKubeEdgeBinary(KubeCloudBinaryName); err != nil {
			return err
		}
	}

	return nil
}
