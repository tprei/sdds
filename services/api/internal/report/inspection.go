package report

// InspectionRow is the operator read model produced by report inspection.
// TargetSummary is nil and TargetMissing is true when the reported content was
// deleted after the report was filed.
type InspectionRow struct {
	ReportPageKey  int64
	ID             string
	CreatedAt      int64
	ReporterUserID string
	TargetType     TargetType
	TargetID       string
	Reason         Reason
	Details        *string
	TargetSummary  *string
	TargetMissing  bool
}
