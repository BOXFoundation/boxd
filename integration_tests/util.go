// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// LoadJSONFromFile load json from file to result
func LoadJSONFromFile(fileName string, result interface{}) error {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return err
	}
	return nil
}

func startLocalNodes(peerCount int) ([]*os.Process, error) {
	var processes []*os.Process
	for i := 0; i < peerCount; i++ {
		cfg := fmt.Sprintf("--config=%s.box-%d.yaml", workDir, i+1)
		args := []string{"../box", "start", cfg, "&"}
		logger.Infof("startLocalNodes: %v", args)
		p, err := StartProcess(args...)
		if err != nil {
			return processes, err
		}
		processes = append(processes, p)
	}
	return processes, nil
}

func stopLocalNodes(processes ...*os.Process) error {
	if len(processes) == 0 {
		return nil
	}
	var wg sync.WaitGroup
	for _, p := range processes {
		wg.Add(1)
		go func(p *os.Process) {
			defer wg.Done()
			if err := p.Signal(os.Interrupt); err != nil {
				logger.Warnf("send Interrupt signal to process[%d] error: %s", p.Pid, err)
				if err := p.Kill(); err != nil {
					logger.Warnf("kill process[%d] error: %s", p.Pid, err)
				}
				return
			}
			s, err := p.Wait()
			if err != nil {
				logger.Warnf("wait process[%d] done error: %s", p.Pid, err)
			}
			logger.Infof("process[%d] %s", p.Pid, s)
		}(p)
	}
	wg.Wait()
	return nil
}

func startNodes() error {
	cmd := exec.Command("docker-compose", "-f", dockerComposeFile, "up",
		"--force-recreate", "-d")
	logger.Infof("exec: docker-compose -f %s up --force-recreate -d",
		dockerComposeFile)
	if err := cmd.Run(); err != nil {
		logger.Infof("docker-compose up failed: %v", err)
		return err
	}
	logger.Infof("docker-compose up succeed!")
	return nil
}

func stopNodes() error {
	cmd := exec.Command("docker-compose", "down")
	logger.Infof("exec: docker-compose down")
	if err := cmd.Run(); err != nil {
		logger.Infof("docker-compose down failed: %v", err)
		return err
	}
	logger.Infof("docker-compose down succeed!")
	return nil
}

func prepareEnv(count int) error {
	logger.Info("prepare test workspace")
	for i := 0; i < count; i++ {
		// remove database and logs directories in ../.devconfig/ ws
		prefix := workDir + "ws" + strconv.Itoa(i+1)
		logger.Infof("clean %s database and logs", prefix)
		if err := os.RemoveAll(prefix + "/database"); err != nil {
			return err
		}
		if err := os.RemoveAll(prefix + "/logs"); err != nil {
			return err
		}
		// check .box-*.yaml exists
		cfgFile := workDir + ".box-" + strconv.Itoa(i+1) + ".yaml"
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			return err
		}
		// check key*.keystore exists
		keyFile := workDir + "keyfile/key" + strconv.Itoa(i+1) + ".keystore"
		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			return err
		}
		logger.Infof("configure file %s and keyfile %s exists", cfgFile, keyFile)
	}
	return nil
}

func tearDown(count int) error {
	// remove database and logs directories in ../.devconfig/ ws
	for i := 0; i < count; i++ {
		prefix := workDir + "ws" + strconv.Itoa(i+1)
		logger.Infof("clean %s database and logs", prefix)
		if err := os.RemoveAll(prefix + "/database"); err != nil {
			logger.Error(err)
		}
		if err := os.RemoveAll(prefix + "/logs"); err != nil {
			logger.Error(err)
		}
	}
	return nil
}

// GetIniKV parses ini buff and get value of key
func GetIniKV(iniBuf, key string) string {
	lineSep, kvSep := "\n", ":"
	lines := strings.Split(iniBuf, lineSep)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		pos := strings.Index(line, kvSep)
		if pos < 0 {
			continue
		}
		if key == strings.TrimSpace(line[:pos]) {
			return strings.TrimSpace(line[pos+len(kvSep):])
		}
	}
	return ""
}

func removeKeystoreFiles(addrs ...string) {
	for _, v := range addrs {
		path := walletDir + v + ".keystore"
		if err := os.Remove(path); err != nil {
			logger.Error(err)
		}
	}
	logger.Infof("remove %d keystore files", len(addrs))
}

// StartProcess starts a process with args and default process attributions
func StartProcess(args ...string) (*os.Process, error) {
	if len(args) == 0 {
		return nil, errors.New("no params to start process")
	}
	var err error
	if args[0], err = exec.LookPath(args[0]); err != nil {
		return nil, err
	}
	var procAttr os.ProcAttr
	//procAttr.Dir = ".."
	procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
	p, err := os.StartProcess(args[0], args, &procAttr)
	if err != nil {
		return nil, err
	}
	return p, nil
}
