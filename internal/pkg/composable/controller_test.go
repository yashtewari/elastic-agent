// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package composable_test

import (
	"context"
	"sync"
	"testing"

	"github.com/elastic/elastic-agent/internal/pkg/core/logger"

	"github.com/elastic/elastic-agent/internal/pkg/agent/transpiler"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-agent/internal/pkg/composable"
	"github.com/elastic/elastic-agent/internal/pkg/config"

	_ "github.com/elastic/elastic-agent/internal/pkg/composable/providers/env"
	_ "github.com/elastic/elastic-agent/internal/pkg/composable/providers/host"
	_ "github.com/elastic/elastic-agent/internal/pkg/composable/providers/local"
	_ "github.com/elastic/elastic-agent/internal/pkg/composable/providers/localdynamic"
)

func TestController(t *testing.T) {
	cfg, err := config.NewConfigFrom(map[string]interface{}{
		"providers": map[string]interface{}{
			"env": map[string]interface{}{
				"enabled": "false",
			},
			"local": map[string]interface{}{
				"vars": map[string]interface{}{
					"key1": "value1",
				},
			},
			"local_dynamic": map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"vars": map[string]interface{}{
							"key1": "value1",
						},
						"processors": []map[string]interface{}{
							{
								"add_fields": map[string]interface{}{
									"fields": map[string]interface{}{
										"add": "value1",
									},
									"to": "dynamic",
								},
							},
						},
					},
					{
						"vars": map[string]interface{}{
							"key1": "value2",
						},
						"processors": []map[string]interface{}{
							{
								"add_fields": map[string]interface{}{
									"fields": map[string]interface{}{
										"add": "value2",
									},
									"to": "dynamic",
								},
							},
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	log, err := logger.New("", false)
	require.NoError(t, err)
	c, err := composable.New(log, cfg)
	require.NoError(t, err)

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg.Add(1)
	var setVars []*transpiler.Vars
	err = c.Run(ctx, func(vars []*transpiler.Vars) {
		setVars = vars
		wg.Done()
	})
	require.NoError(t, err)
	wg.Wait()

	assert.Len(t, setVars, 3)

	_, hostExists := setVars[0].Lookup("host")
	assert.True(t, hostExists)
	_, envExists := setVars[0].Lookup("env")
	assert.False(t, envExists)
	local, _ := setVars[0].Lookup("local")
	localMap := local.(map[string]interface{})
	assert.Equal(t, "value1", localMap["key1"])

	local, _ = setVars[1].Lookup("local_dynamic")
	localMap = local.(map[string]interface{})
	assert.Equal(t, "value1", localMap["key1"])

	local, _ = setVars[2].Lookup("local_dynamic")
	localMap = local.(map[string]interface{})
	assert.Equal(t, "value2", localMap["key1"])
}