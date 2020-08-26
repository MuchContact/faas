// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package handlers

import (
	"sync"
	"testing"
	"time"
)

func TestAutoScale(t *testing.T) {
	var mmap sync.Map
	mmap.Store("key1", time.Now())

	val, _ := mmap.Load("key1")

	status := time.Since(val.(time.Time)).Minutes() > 5

	t.Log(status)

	mmap.Range(func(key, value interface{}) bool {
		t.Log(key.(string) + ":" + value.(string))
		return true
	})

}
