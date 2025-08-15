/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package aicontext

import (
	"os"
	"strconv"
)

// AnthropicConfig represents the configuration system, mirroring Python config.py
// This provides centralized configuration for AI Gateway functionality
type AnthropicConfig struct {
	// Token limits configuration
	MinTokensLimit int
	MaxTokensLimit int

	// Model mapping configuration
	SmallModel  string
	MiddleModel string
	BigModel    string
}

var GlobalAnthropicConfig = NewAnthropicConfig()

func NewAnthropicConfig() *AnthropicConfig {
	return &AnthropicConfig{
		// Token limits - matching Python constants
		MinTokensLimit: getEnvAsInt("MIN_TOKENS_LIMIT", 12800),
		MaxTokensLimit: getEnvAsInt("MAX_TOKENS_LIMIT", 128000),

		// Model mappings - can be overridden by environment
		SmallModel:  getEnv("SMALL_MODEL", "gpt-5"),
		MiddleModel: getEnv("MIDDLE_MODEL", "gpt-5"),
		BigModel:    getEnv("BIG_MODEL", "gpt-5"),
	}
}

// GetSmallModel returns the small model mapping
func (c *AnthropicConfig) GetSmallModel() string {
	return c.SmallModel
}

// GetMiddleModel returns the middle model mapping
func (c *AnthropicConfig) GetMiddleModel() string {
	return c.MiddleModel
}

// GetBigModel returns the big model mapping
func (c *AnthropicConfig) GetBigModel() string {
	return c.BigModel
}

// GetMinTokensLimit returns the minimum token limit
func (c *AnthropicConfig) GetMinTokensLimit() int {
	return c.MinTokensLimit
}

// GetMaxTokensLimit returns the maximum token limit
func (c *AnthropicConfig) GetMaxTokensLimit() int {
	return c.MaxTokensLimit
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
