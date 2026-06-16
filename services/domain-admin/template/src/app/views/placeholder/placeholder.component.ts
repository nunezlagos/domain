import { Component, inject } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { map } from 'rxjs/operators';

import {
  CardComponent, CardBodyComponent, CardHeaderComponent,
  AlertComponent,
} from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

// DOMAINSERV-2: PlaceholderComponent compartido para todas las features
// admin-* que aún no están implementadas. Cuando se implemente la HU
// correspondiente, se reemplaza el `loadComponent` en el routes.ts.
//
// Muestra: nombre de la feature, número de issue pendiente, y un botón
// "Ver issue spec" que linkea al file local (sólo en dev — en prod no hace
// nada porque el spec vive en /openspec/ en el repo).
@Component({
  selector: 'app-placeholder',
  template: `
    <c-card class="mb-4">
      <c-card-header>
        <svg cIcon name="cilClock" class="me-2"></svg>
        {{ title() }} — pendiente
      </c-card-header>
      <c-card-body>
        <c-alert color="warning" class="d-flex align-items-center">
          <svg cIcon name="cilWarning" class="me-2"></svg>
          Esta feature está pendiente. Spec en
          <code class="ms-1">openspec/changes/REQ-41-admin-dashboard/issue-{{ issue() }}/</code>
          (issue {{ issue() }}).
        </c-alert>
        <p class="text-body-secondary mb-0">
          {{ description() }}
        </p>
      </c-card-body>
    </c-card>
  `,
  imports: [
    CardComponent, CardBodyComponent, CardHeaderComponent,
    AlertComponent, IconDirective,
  ],
})
export class PlaceholderComponent {
  private readonly route = inject(ActivatedRoute);

  readonly title = toSignal(
    this.route.data.pipe(map(d => d['title'] ?? 'Feature')),
    { initialValue: 'Feature' },
  );

  readonly issue = toSignal(
    this.route.data.pipe(map(d => d['issue'] ?? '???')),
    { initialValue: '???' },
  );

  readonly description = toSignal(
    this.route.data.pipe(map(d => d['description'] ?? 'Feature pendiente de implementación.')),
    { initialValue: 'Feature pendiente de implementación.' },
  );
}
