package api

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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/panther-labs/panther/api/lambda/source/models"
)

func TestAPI_ListLogTypes(t *testing.T) {
	expectedLogTypes := []string{"one", "two"}
	listOutput := []*models.SourceIntegration{
		{
			SourceIntegrationMetadata: models.SourceIntegrationMetadata{
				IntegrationType: models.IntegrationTypeAWS3,
				LogTypes:        []string{"one"},
			},
		},
		{
			SourceIntegrationMetadata: models.SourceIntegrationMetadata{
				IntegrationType: models.IntegrationTypeSqs,
				LogTypes:        []string{"one", "two"}, // "one" is duplicate with above
			},
		},
		// FIXME: This was changed for cloudsecurity feature branch
		//{ // should not match
		//	SourceIntegrationMetadata: models.SourceIntegrationMetadata{
		//		IntegrationType: models.IntegrationTypeAWSScan,
		//		LogTypes:        nil,
		//	},
		//},
	}
	assert.Equal(t, expectedLogTypes, collectLogTypes(listOutput))
}
