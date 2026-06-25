package main

import "nunezlagos/domain/internal/service/issuebuilder"

// wireServices resuelve las dependencias cruzadas post-construcción.
// Estas asignaciones no pueden hacerse durante buildServices porque crean
// ciclos entre servicios construidos en pasos consecutivos.
func wireServices(s *serverServices) {
	// outboundEmitter depende de usageAlertsService; obsService ya existe
	// cuando se construye emitter — la asignación inversa se hace aquí.
	s.ObsService.Events = s.OutboundEmitter

	// issuebuilderSvc requiere requirementService y huService que se
	// construyen después en buildServices (S3 primero, luego estos).
	if s.AttachmentService != nil {
		s.IssuebuilderSvc.Attachments = &issuebuilder.AttachmentServiceAdapter{
			Inner: s.AttachmentService,
		}
	}
	s.IssuebuilderSvc.ReqSvc = &issuebuilder.RequirementServiceAdapter{
		Inner: s.RequirementService,
	}
	s.IssuebuilderSvc.IssueSvc = &issuebuilder.IssueServiceAdapter{
		Inner: s.HUService,
	}
}
