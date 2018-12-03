// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

// CfgData define config data responding conf.json
type CfgData map[string]*json.RawMessage

var cfgData CfgData

// InitSpecConf inits config and return CfgData variable
func InitSpecConf(cfgFile string) (CfgData, error) {
	cfgBytes, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read json file: %s, error: %s", cfgFile, err)
	}
	raw := make(CfgData)
	err = json.Unmarshal(cfgBytes, &raw)
	if err != nil {
		return nil, fmt.Errorf("failed to Unmarshal file: %s, error: %s", cfgFile, err)
	}
	return raw, nil
}

// InitConf init config and store them int default variable cfgData
func InitConf(cfgFile string) error {
	raw, err := InitSpecConf(cfgFile)
	if err != nil {
		return err
	}
	cfgData = raw
	return nil
}

// GetSpecCfgVal get special cfg value from param cfgdata
func GetSpecCfgVal(cfgData CfgData, def interface{}, keys ...string) (
	v interface{}, err error) {
	v = def
	var m interface{}
	m = cfgData
	for i, k := range keys {
		if mm, ok := m.(CfgData); ok {
			if d, ok := mm[k]; ok {
				if i == len(keys)-1 {
					if err1 := json.Unmarshal(*d, &v); err1 != nil {
						err = fmt.Errorf("failed to Unmarshal config, key: %v, error: %s",
							keys, err1)
					}
					if _, ok := def.(int); ok {
						v = int(v.(float64))
					}
					return
				}
				mm = map[string]*json.RawMessage{}
				if err1 := json.Unmarshal(*d, &mm); err1 != nil {
					err = fmt.Errorf("failed to Unmarshal config, key: %v, error: %s",
						keys, err1)
					return
				}
				m = mm
			} else {
				err = fmt.Errorf("cfgData has no key[%s]", k)
				return
			}
		} else {
			err = fmt.Errorf("cfgData wrong value for key[%s]", k)
			return
		}
	}
	err = fmt.Errorf("GetCfgVal error: invalid Key: %v", keys)
	return
}

// GetCfgVal get value from default cfgData
func GetCfgVal(def interface{}, keys ...string) (interface{}, error) {
	return GetSpecCfgVal(cfgData, def, keys...)
}

// GetStringSpecCfgVal get string cfg value
func GetStringSpecCfgVal(cfgData CfgData, def interface{}, keys ...string) (
	string, error) {
	v, err := GetSpecCfgVal(cfgData, def, keys...)
	if err != nil {
		return "", err
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("get string cfg value[%v] assert failed, origin: %v",
			keys, v)
	}
	return s, nil
}

// GetStringCfgVal get string cfg value
func GetStringCfgVal(def interface{}, keys ...string) (string, error) {
	return GetStringSpecCfgVal(cfgData, def, keys...)
}

// GetIntSpecCfgVal get int config vlaue
func GetIntSpecCfgVal(cfgData CfgData, def interface{}, keys ...string) (int, error) {
	v, err := GetSpecCfgVal(cfgData, def, keys...)
	if err != nil {
		return 0, err
	}
	i, ok := v.(int)
	if !ok {
		return 0, fmt.Errorf("get int cfg value[%v] assert failed, origin: %v",
			keys, v)
	}
	return i, nil
}

// GetIntCfgVal get int config value
func GetIntCfgVal(def interface{}, keys ...string) (int, error) {
	return GetIntSpecCfgVal(cfgData, def, keys...)
}

// GetBoolSpecCfgVal get bool value
func GetBoolSpecCfgVal(cfgData CfgData, def interface{}, keys ...string) (bool, error) {
	v, err := GetSpecCfgVal(cfgData, def, keys...)
	if err != nil {
		return false, err
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("get bool cfg value[%v] assert failed, origin: %v",
			keys, v)
	}
	return b, nil
}

// GetBoolCfgVal get bool value
func GetBoolCfgVal(def interface{}, keys ...string) (bool, error) {
	return GetBoolSpecCfgVal(cfgData, def, keys...)
}

// GetFloatSpecCfgVal get float value
func GetFloatSpecCfgVal(cfgData CfgData, def interface{}, keys ...string) (float64, error) {
	v, err := GetSpecCfgVal(cfgData, def, keys...)
	if err != nil {
		return 0.0, err
	}
	f, ok := v.(float64)
	if !ok {
		return 0.0, fmt.Errorf("get float cfg value[%v] assert failed, origin: %v",
			keys, v)
	}
	return f, nil
}

// GetFloatCfgVal get float value
func GetFloatCfgVal(def interface{}, keys ...string) (float64, error) {
	return GetFloatSpecCfgVal(cfgData, def, keys...)
}

// GetStrArraySpecCfgVal get string array value
func GetStrArraySpecCfgVal(cfgData CfgData, def interface{}, keys ...string) ([]string, error) {
	v, err := GetSpecCfgVal(cfgData, def, keys...)
	if err != nil {
		return nil, err
	}
	ss, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("value[%+v] type assertion failed", v)
	}
	return conv2StrSlice(ss), nil
}

// GetStrArrayCfgVal get string array value
func GetStrArrayCfgVal(def interface{}, keys ...string) ([]string, error) {
	return GetStrArraySpecCfgVal(cfgData, def, keys...)
}

// GetFloatArraySpecCfgVal get float array value
func GetFloatArraySpecCfgVal(cfgData CfgData, def interface{}, keys ...string) ([]float64, error) {
	v, err := GetSpecCfgVal(cfgData, def, keys...)
	if err != nil {
		return nil, err
	}
	ss, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("value[%+v] type assertion failed", v)
	}
	ff := conv2FloatSlice(ss)
	if ff == nil {
		return nil, fmt.Errorf("convert value for keys[%v] to []float64 error", keys)
	}
	return ff, nil
}

// GetFloatArrayCfgVal get float array value
func GetFloatArrayCfgVal(def interface{}, keys ...string) ([]float64, error) {
	return GetFloatArraySpecCfgVal(cfgData, def, keys...)
}

// GetIntArraySpecCfgVal get int array value
func GetIntArraySpecCfgVal(cfgData CfgData, def interface{}, keys ...string) ([]int, error) {
	ff, err := GetFloatArraySpecCfgVal(cfgData, def, keys...)
	if err != nil {
		return nil, err
	}
	ints := make([]int, len(ff))
	for i, v := range ff {
		ints[i] = int(v)
	}
	return ints, nil
}

// GetIntArrayCfgVal get int array value
func GetIntArrayCfgVal(def interface{}, keys ...string) ([]int, error) {
	return GetIntArraySpecCfgVal(cfgData, def, keys...)
}

func conv2StrSlice(si []interface{}) []string {
	var ss []string
	for _, v := range si {
		ss = append(ss, fmt.Sprint(v))
	}
	return ss
}

func conv2FloatSlice(fi []interface{}) []float64 {
	var ff []float64
	for _, v := range fi {
		if f, ok := v.(float64); ok {
			ff = append(ff, f)
		} else {
			return nil
		}
	}
	return ff
}
