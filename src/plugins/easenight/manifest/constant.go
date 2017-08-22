/**
 * Created by g7tianyi on 14/08/2017
 */

package manifest

const (
	DEFAULT_ENVIRONMENT = "general"
)

// Manifest keys withhold by system default
const (
	ENV           = "env"
	NAME          = "name"
	SHELL         = "shell"  // a simple, one-line bash command
	SCRIPT        = "script" // path to a user-defined script
	TIMEOUT       = "timeout"
	FILES         = "files"
	TEMPLATES     = "templates"
	APPROVALS     = "approvals"
	VARIABLES     = "variables"
	NOTIFICATIONS = "notifications"
	BEFORE_RUN    = "before_run"
	AFTER_RUN     = "after_run"
)

// Manifest keys for `Build`
const (
	// Build Image
	DOCKER_REGISTRY         = "docker_registry"
	DOCKER_REGISTRY_ID      = "docker_registry_id"
	DOCKER_FILE             = "docker_file"
	IMAGE_NAME              = "image_name"
	PUSH_IMAGE              = "push_image"
	EASE_STACK              = "ease_stack"
	EASE_STACK_START        = "ease_stack_start"
	EASE_STACK_STOP         = "ease_stack_stop"
	EASE_STACK_APPLY_CONFIG = "ease_stack_apply_config"
	EASE_STACK_HEALTH_CHECK = "ease_stack_health_check"
	// Build App
	APP_NAME     = "app_name"
	APP_LOCATION = "app_location"
)

// Task keys withhold by system trace
const (
	PIPELINE_MODE = "pipeline_mode"
	INSTANCES     = "instances"
	STATUS_CODE   = "status_code"
	STATUS_NAME   = "status_name"

	NIGHTWATCH_SERVER = "easewatch_endpoint"
	ANSIBLE_DIR       = "ansible_dir"
	PIPELINE_ID       = "pipeline_id"
	SCHEDULE_ID       = "schedule_id"
	PLUGIN_NAME       = "plugin_name"
	REPO_URL          = "repo_url"
	REPO_NAME         = "repo_name"
	REPO_EVENT_ID     = "repo_event_id"
	PUSH_BY           = "push_by"
	PUSH_ID           = "push_id" // The combination of commit IDs
	PUSH_MESSAGE      = "push_message"
	IMAGE_ID          = "image_id"
	IMAGE_VERSION     = "image_version"
	APP_ID            = "app_id"
	APP_VERSION       = "app_version"

	GITHUB_USERNAME     = "github_username"
	GITHUB_PASSWORD     = "github_password"
	GITHUB_ACCESS_TOKEN = "github_access_token"
	GITHUB_SECRET_TOKEN = "github_secret_token"
)
