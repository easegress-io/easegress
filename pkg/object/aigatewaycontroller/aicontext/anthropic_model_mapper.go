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
	"strings"
)

// Global instance following the Python pattern: model_manager = ModelManager(config)
var GlobalModelManager = NewDefaultModelManager()

// DefaultModelManager implements the ModelManager interface
// This is a precise translation of the Python ModelManager class
type DefaultModelManager struct {
	config *AnthropicConfig
}

// NewDefaultModelManager creates a new DefaultModelManager with the given configuration
func NewDefaultModelManager() *DefaultModelManager {
	return &DefaultModelManager{
		config: GlobalAnthropicConfig,
	}
}

// NewModelManagerWithConfig creates a new DefaultModelManager with custom configuration
func NewModelManagerWithConfig(config *AnthropicConfig) *DefaultModelManager {
	return &DefaultModelManager{
		config: config,
	}
}

// MapClaudeModelToOpenAI maps Claude model names to OpenAI model names based on BIG/SMALL pattern
// This is a precise translation of the Python map_claude_model_to_openai method
func (m *DefaultModelManager) MapClaudeModelToOpenAI(claudeModel string) string {
	// If it's already an OpenAI model, return as-is
	if strings.HasPrefix(claudeModel, "gpt-") || strings.HasPrefix(claudeModel, "o1-") {
		return claudeModel
	}

	// If it's other supported models (ARK/Doubao/DeepSeek), return as-is
	if strings.HasPrefix(claudeModel, "ep-") ||
		strings.HasPrefix(claudeModel, "doubao-") ||
		strings.HasPrefix(claudeModel, "deepseek-") {
		return claudeModel
	}

	// Map based on model naming patterns
	modelLower := strings.ToLower(claudeModel)
	if strings.Contains(modelLower, "haiku") {
		return m.config.GetSmallModel()
	} else if strings.Contains(modelLower, "sonnet") {
		return m.config.GetMiddleModel()
	} else if strings.Contains(modelLower, "opus") {
		return m.config.GetBigModel()
	}
	// Default to big model for unknown models
	return m.config.GetBigModel()
}
