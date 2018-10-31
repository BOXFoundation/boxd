package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
)

// KeyStore defines key structure
type KeyStore struct {
	Address string `json:"address"`
}

func minerAddress(index int) (string, error) {
	file := keyDir + fmt.Sprintf("key%d.keystore", index+1)
	var ks KeyStore
	if err := LoadJSONFromFile(file, &ks); err != nil {
		return "", err
	}
	return ks.Address, nil
}

func newAccount() (string, error) {
	nodeName := "boxd_node_1_1"
	args := []string{"exec", nodeName, "./newaccount.sh"}
	cmd := exec.Command("docker", args...)
	logger.Infof("exec: docker %v", args)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(stdout)
	if err != nil {
		return "", err
	}
	//logger.Infof("newAccount from container %s return: %s", nodeName, buf.String())
	if err := cmd.Wait(); err != nil {
		return "", err
	}
	addr := GetIniKV(buf.String(), "Address")
	if addr == "" {
		return "", errors.New("newAccount failed, address is empty")
	}
	return addr, nil
}
