package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
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
	for i := 0; i < count; i++ {
		// remove database and logs directories in ../.devconfig/ ws
		prefix := workDir + "ws" + strconv.Itoa(i)
		if err := os.RemoveAll(prefix + "/database"); err != nil {
			return err
		}
		if err := os.RemoveAll(prefix + "/logs"); err != nil {
			return err
		}
		// check .box-*.yaml exists
		cfgFile := workDir + ".box-" + strconv.Itoa(i) + ".yaml"
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func tearDown(count int) error {
	for i := 0; i < count; i++ {
		// remove database and logs directories in ../.devconfig/ ws
		prefix := workDir + "ws" + strconv.Itoa(i)
		if err := os.RemoveAll(prefix + "/database"); err != nil {
			return err
		}
		if err := os.RemoveAll(prefix + "/logs"); err != nil {
			return err
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
