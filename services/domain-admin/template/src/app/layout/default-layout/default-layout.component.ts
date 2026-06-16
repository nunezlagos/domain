import { Component, computed, inject, signal } from '@angular/core';
import { RouterLink, RouterOutlet } from '@angular/router';
import { NgScrollbar } from 'ngx-scrollbar';

import { IconDirective } from '@coreui/icons-angular';
import {
  ContainerComponent,
  ShadowOnScrollDirective,
  SidebarBrandComponent,
  SidebarComponent,
  SidebarFooterComponent,
  SidebarHeaderComponent,
  SidebarNavComponent,
  SidebarToggleDirective,
  SidebarTogglerDirective
} from '@coreui/angular';

import { DefaultFooterComponent, DefaultHeaderComponent } from './';
import { navAdminItems, navPlatformItems } from './_nav';
import { AuthService } from '../../core/auth.service';

@Component({
  selector: 'app-dashboard',
  templateUrl: './default-layout.component.html',
  styleUrls: ['./default-layout.component.scss'],
  imports: [
    SidebarComponent,
    SidebarHeaderComponent,
    SidebarBrandComponent,
    SidebarNavComponent,
    SidebarFooterComponent,
    SidebarToggleDirective,
    SidebarTogglerDirective,
    ContainerComponent,
    DefaultFooterComponent,
    DefaultHeaderComponent,
    IconDirective,
    NgScrollbar,
    RouterOutlet,
    RouterLink,
    ShadowOnScrollDirective
  ]
})
export class DefaultLayoutComponent {
  private readonly auth = inject(AuthService);

  // HU-41.1: items del sidebar. Si el rol activo es super_admin, se
  // agrega la sección "Plataforma" (Cross-org) al final.
  readonly navItems = computed(() => {
    const isSuperAdmin = this.auth.activeRole()?.slug === 'super_admin';
    return isSuperAdmin
      ? [...navAdminItems, ...navPlatformItems]
      : [...navAdminItems];
  });

  constructor() {
    // HU-41.1: al montar el layout, si hay token en localStorage, hidrata
    // los signals del AuthService (user, activeRole, roles) desde el backend.
    // Si el token está expirado, el interceptor maneja el 401.
    void this.auth.refreshFromMe();
  }
}
