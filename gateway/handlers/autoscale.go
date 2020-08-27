// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/openfaas/faas/gateway/metrics"
	"github.com/openfaas/faas/gateway/scaling"
)

var cooldownMap sync.Map

// MakeAlertHandler handles alerts from Prometheus Alertmanager
func MakeAutoScaleHandler(service scaling.ServiceQuery, prometheusQuery metrics.PrometheusQueryFetcher, defaultNamespace string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cooldownMap.Range(func(key, value interface{}) bool {
			log.Printf("[AutoScale] function=%s cooldownStart: %s\n", key.(string), value.(time.Time).String())
			return true
		})

		errors := handleAutoScale(prometheusQuery, service, defaultNamespace)
		if len(errors) > 0 {
			log.Println("[AutoScale] errors: ")
			log.Println(errors)
			var errorOutput string
			for d, err := range errors {
				errorOutput += fmt.Sprintf("[AutoScale] [%d] %s\n", d, err)
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
		log.Printf("[AutoScale] get prometheus metric : %s %s \n", functionName, value)

		// record 0 timestamp
		if parsedVal == 0 {
			if _, exists := cooldownMap.Load(functionName); !exists {
				cooldownMap.Store(functionName, time.Now())
			}
		} else {
			if _, exists := cooldownMap.Load(functionName); exists {
				cooldownMap.Delete(functionName)
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
	log.Printf("[AutoScale] reading replicas for function=%s.%s\n", serviceName, namespace)
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
		} else {
			err = getErr
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
	val, exists := cooldownMap.Load(functionName)
	if exists {
		return time.Since(val.(time.Time)).Minutes() > 5
	}
	return false
}
