package outputs

/**
 * Panther is a Cloud-Native SIEM for the Modern Security Team.
 * Copyright (C) 2020 Panther Labs Inc
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	jsoniter "github.com/json-iterator/go"

	alertModels "github.com/panther-labs/panther/api/lambda/delivery/models"
	outputModels "github.com/panther-labs/panther/api/lambda/outputs/models"
	"github.com/panther-labs/panther/pkg/genericapi"
)

var (
	policyURLPrefix = os.Getenv("POLICY_URL_PREFIX")
	appDomainURL    = os.Getenv("APP_DOMAIN_URL")
	alertURLPrefix  = os.Getenv("ALERT_URL_PREFIX")
)

// HTTPWrapper encapsulates the Golang's http client
type HTTPWrapper struct {
	httpClient HTTPiface
}

// PostInput type
type PostInput struct {
	url     string
	body    interface{}
	headers map[string]string
}

// HTTPWrapperiface is the interface for our wrapper around Golang's http client
type HTTPWrapperiface interface {
	post(*PostInput) *AlertDeliveryResponse
}

// HTTPiface is an interface for http.Client to simplify unit testing.
type HTTPiface interface {
	Do(*http.Request) (*http.Response, error)
}

// API is the interface for output delivery that can be used for mocks in tests.
type API interface {
	Slack(*alertModels.Alert, *outputModels.SlackConfig) *AlertDeliveryResponse
	PagerDuty(*alertModels.Alert, *outputModels.PagerDutyConfig) *AlertDeliveryResponse
	Github(*alertModels.Alert, *outputModels.GithubConfig) *AlertDeliveryResponse
	Jira(*alertModels.Alert, *outputModels.JiraConfig) *AlertDeliveryResponse
	Opsgenie(*alertModels.Alert, *outputModels.OpsgenieConfig) *AlertDeliveryResponse
	MsTeams(*alertModels.Alert, *outputModels.MsTeamsConfig) *AlertDeliveryResponse
	Sqs(*alertModels.Alert, *outputModels.SqsConfig) *AlertDeliveryResponse
	Sns(*alertModels.Alert, *outputModels.SnsConfig) *AlertDeliveryResponse
	Asana(*alertModels.Alert, *outputModels.AsanaConfig) *AlertDeliveryResponse
	CustomWebhook(*alertModels.Alert, *outputModels.CustomWebhookConfig) *AlertDeliveryResponse
}

// OutputClient encapsulates the clients that allow sending alerts to multiple outputs
type OutputClient struct {
	// WARNING: This is shared by concurrent goroutines.
	// Do not mutate any fields in the goroutines, and do not use maps without proper locking.
	session     *session.Session // safe for concurrent reads, not writes
	httpWrapper HTTPWrapperiface
}

// OutputClient must satisfy the API interface.
var _ API = (*OutputClient)(nil)

// New creates a new client for alert delivery.
func New(sess *session.Session) *OutputClient {
	return &OutputClient{
		session:     sess,
		httpWrapper: &HTTPWrapper{httpClient: &http.Client{}},
	}
}

// The default payload delivered by all outputs to destinations
// Each destination can augment this with its own custom fields.
// This struct intentionally never uses the `omitempty` attribute as we want to keep the keys even
// if they have `null` fields. However, we need to ensure there are no `null` arrays or
// objects.
type Notification struct {
	// [REQUIRED] The Policy or Rule ID
	ID string `json:"id"`

	// [REQUIRED] The timestamp (RFC3339) of the alert at creation.
	CreatedAt time.Time `json:"createdAt"`

	// [REQUIRED] The severity enum of the alert set in Panther UI. Will be one of INFO LOW MEDIUM HIGH CRITICAL.
	Severity string `json:"severity"`

	// [REQUIRED] The Type enum if an alert is for a rule or policy. Will be one of RULE POLICY.
	Type string `json:"type"`

	// [REQUIRED] Link to the alert in Panther UI
	Link string `json:"link"`

	// [REQUIRED] The title for this notification
	Title string `json:"title"`

	// [REQUIRED] The Name of the Rule or Policy
	Name *string `json:"name"`

	// An AlertID that was triggered by a Rule. It will be `null` in case of policies
	AlertID *string `json:"alertId"`

	// An AlertContext
	AlertContext map[string]interface{} `json:"alertContext"`

	// The Description of the rule set in Panther UI
	Description *string `json:"description"`

	// The Runbook is the user-provided triage information set in Panther UI
	Runbook *string `json:"runbook"`

	// Tags is the set of policy tags set in Panther UI
	Tags []string `json:"tags"`

	// Version is the S3 object version for the policy
	Version *string `json:"version"`
}

func generateNotificationFromAlert(alert *alertModels.Alert) Notification {
	notification := Notification{
		ID:           alert.AnalysisID,
		AlertID:      alert.AlertID,
		Name:         alert.AnalysisName,
		Severity:     alert.Severity,
		Type:         alert.Type,
		Link:         generateURL(alert),
		Title:        generateAlertTitle(alert),
		Description:  alert.AnalysisDescription,
		Runbook:      alert.Runbook,
		Tags:         alert.Tags,
		Version:      alert.Version,
		CreatedAt:    alert.CreatedAt,
		AlertContext: alert.Context,
	}

	genericapi.ReplaceMapSliceNils(&notification)
	return notification
}

func generateAlertMessage(alert *alertModels.Alert) string {
	switch alert.Type {
	case alertModels.RuleType:
		return getDisplayName(alert) + " triggered"
	case alertModels.RuleErrorType:
		return getDisplayName(alert) + " encountered an error"
	case alertModels.PolicyType:
		return getDisplayName(alert) + " failed on new resources"
	default:
		panic("uknown alert type " + alert.Type)
	}
}

func generateDetailedAlertMessage(alert *alertModels.Alert) string {
	const detailedMessageTemplate = "%s\nFor more details please visit: %s\nSeverity: %s\nRunbook: %s\nDescription: %s\nAlertContext: %s"
	// Best effort to marshal alert context
	marshaledContext, _ := jsoniter.MarshalToString(alert.Context)

	return fmt.Sprintf(
		detailedMessageTemplate,
		generateAlertMessage(alert),
		generateURL(alert),
		alert.Severity,
		aws.StringValue(alert.Runbook),
		aws.StringValue(alert.AnalysisDescription),
		marshaledContext,
	)
}

func generateAlertTitle(alert *alertModels.Alert) string {
	if alert.IsResent {
		return "[Re-sent]: " + *alert.Title
	}
	switch alert.Type {
	case alertModels.RuleType:
		if alert.Title != nil {
			return "New Alert: " + *alert.Title
		}
		return "New Alert: " + getDisplayName(alert)
	case alertModels.RuleErrorType:
		return "New rule error: " + *alert.Title
	case alertModels.PolicyType:
		return "Policy Failure: " + getDisplayName(alert)
	default:
		panic("uknown alert type " + alert.Type)
	}
}

func getDisplayName(alert *alertModels.Alert) string {
	if aws.StringValue(alert.AnalysisName) != "" {
		return *alert.AnalysisName
	}
	return alert.AnalysisID
}

func generateURL(alert *alertModels.Alert) string {
	if alert.IsTest {
		return appDomainURL
	}
	switch alert.Type {
	case alertModels.RuleType, alertModels.RuleErrorType:
		return alertURLPrefix + *alert.AlertID
	case alertModels.PolicyType:
		return policyURLPrefix + alert.AnalysisID
	default:
		panic("uknown alert type" + alert.Type)
	}
}
