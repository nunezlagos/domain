import { Component } from '@angular/core';
import { FooterComponent } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

@Component({
  selector: 'app-default-footer',
  templateUrl: './default-footer.component.html',
  styleUrls: ['./default-footer.component.scss'],
  imports: [IconDirective],
})
export class DefaultFooterComponent extends FooterComponent {
  // HU-41.1: versión hardcodeada por ahora (matches package.json: 5.6.24).
  // En F4+ se puede tomar de build-time (reemplazo de token en el
  // docker-entrypoint que escribe window.__DOMAIN_ENV__.APP_VERSION).
  readonly version = '5.6.24';
  readonly statusUrl = '/healthz';

  constructor() {
    super();
  }
}
