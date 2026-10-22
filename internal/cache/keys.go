package cache

import (
	"fmt"

	"github.com/google/uuid"
)

func SchemeKey(solutionID uuid.UUID, gen int64, schemeXMLID string) string {
	return fmt.Sprintf("cs:v1:scheme:%s:%d:%s", solutionID.String(), gen, schemeXMLID)
}

func SchemeObjectsKey(solutionID uuid.UUID, gen int64, schemeXMLID string) string {
	return fmt.Sprintf("cs:v1:scheme_objects:%s:%d:%s", solutionID.String(), gen, schemeXMLID)
}

func ComplianceKey(userID uuid.UUID, rulesGen int64, solutionID uuid.UUID, solGen int64, scope string) string {
	return fmt.Sprintf("cs:v1:compliance:%s:%d:%s:%d:%s", userID.String(), rulesGen, solutionID.String(), solGen, scope)
}

func SolutionGenKey(solutionID uuid.UUID) string {
	return fmt.Sprintf("cs:v1:gen:%s", solutionID.String())
}

func RulesGenKey(userID uuid.UUID) string {
	return fmt.Sprintf("cs:v1:rulesgen:%s", userID.String())
}

func ComplianceScope(projectXMLID, schemeXMLID string) string {
	if schemeXMLID != "" {
		return "s:" + schemeXMLID
	}
	if projectXMLID != "" {
		return "p:" + projectXMLID
	}
	return "all"
}
