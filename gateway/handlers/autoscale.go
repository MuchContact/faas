// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/openfaas/faas/gateway/metrics"
	"github.com/openfaas/faas/gateway/scaling"
)

var cooldownMap = make(map[string]time.Time)

// MakeAlertHandler handles alerts from Prometheus Alertmanager
func MakeAutoScaleHandler(service scaling.ServiceQuery, prometheusQuery metrics.PrometheusQueryFetcher, defaultNamespace string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for key, val := range cooldownMap {
			log.Printf("[AutoScale] function=%s cooldownStart: %s\n", key, val.String())
		}
		errors := handleAutoScale(prometheusQuery, service, defaultNamespace)
		if len(errors) > 0 {
			log.Println(errors)
			var errorOutput string
			for d, err := range errors {
				errorOutput += fmt.Sprintf("[%d] %s\n", d, err)
			}
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(errorOutput))
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func handleAutoScale(queryFetcher metrics.PrometheusQueryFetcher, service scaling.ServiceQuery, defaultNamespace string) []error {
	var errors []error
	fetch, err := queryFetcher.Fetch("undone_task_num")
	if err != nil {
		errors = append(errors, err)
		return errors
	}
	for _, metric := range fetch.Data.Result {
		functionName := metric.Metric.FunctionName

		value := metric.Value[1]
		parsedVal, _ := strconv.ParseUint(value.(string), 10, 64)

		// record 0 timestamp
		if parsedVal == 0 {
			if _, exists := cooldownMap[functionName]; !exists {
				cooldownMap[functionName] = time.Now()
			}
		} else {
			if _, exists := cooldownMap[functionName]; exists {
				delete(cooldownMap, functionName)
			}
		}

		if err := autoScaleService(functionName, parsedVal, service, defaultNamespace); err != nil {
			log.Println(err)
			errors = append(errors, err)
		}
	}

	return errors
}

func autoScaleService(functionName string, undoneTaskNum uint64, service scaling.ServiceQuery, defaultNamespace string) error {
	var err error

	serviceName, namespace := getNamespace(defaultNamespace, functionName)

	if len(serviceName) > 0 {
		queryResponse, getErr := service.GetReplicas(serviceName, namespace)
		if getErr == nil {

			newReplicas := calculateReplicas(functionName, undoneTaskNum, queryResponse.Replicas, uint64(queryResponse.MaxReplicas), queryResponse.MinReplicas, queryResponse.ScalingFactor)

			log.Printf("[AutoScale] function=%s %d => %d.\n", functionName, queryResponse.Replicas, newReplicas)
			if newReplicas == queryResponse.Replicas {
				return nil
			}

			updateErr := service.SetReplicas(serviceName, namespace, newReplicas)
			if updateErr != nil {
				err = updateErr
			}
		}
	}
	return err
}

// CalculateReplicas decides what replica count to set depending on current/desired amount
func calculateReplicas(functionName string, undoneTaskNum uint64, currentReplicas uint64, maxReplicas uint64, minReplicas uint64, scalingFactor uint64) uint64 {
	var newReplicas = currentReplicas

	//step := uint64(math.Ceil(float64(maxReplicas) / 100 * float64(scalingFactor)))

	if undoneTaskNum > currentReplicas {
		if undoneTaskNum > maxReplicas {
			newReplicas = maxReplicas
		} else {
			newReplicas = undoneTaskNum
		}
	} else if undoneTaskNum <= 0 && cooldownTimeout(functionName) {
		newReplicas = minReplicas
	}

	return newReplicas
}

// 判断函数是否过了静默期
func cooldownTimeout(functionName string) bool {
	val, exists := cooldownMap[functionName]
	if exists {
		return time.Since(val).Minutes() > 5
	}
	return false
}
