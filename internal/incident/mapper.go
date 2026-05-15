package incident

func ToDTO(
	inc Incident,
) IncidentDTO {

	return IncidentDTO{
		ID: inc.ID,

		Title: inc.Title,

		Status: string(inc.Status),

		Severity: inc.Severity,

		ResourceKind: inc.ResourceKind,

		ResourceName: inc.ResourceName,

		StartedAt: inc.StartTime,

		UpdatedAt: inc.UpdatedAt,
	}
}