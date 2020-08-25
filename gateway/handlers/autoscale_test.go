// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package handlers

import (
	"testing"
	"time"
)

func TestAutoScale(t *testing.T) {
	var cooldownMap = make(map[string]time.Time)
	cooldownMap["a"] = time.Now()
	cooldownMap["c"] = time.Now()
	//if _, ok := cooldownMap["a"]; ok {
	//	t.Log(cooldownMap["a"])
	//}
	for key, val := range cooldownMap {
		t.Logf("[AutoScale] function=%s cooldownStart: %s\n", key, val.String())
	}
}
