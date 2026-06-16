import { NgTemplateOutlet, NgIf } from '@angular/common';
import { Component, computed, inject, input, signal } from '@angular/core';
import { Router, RouterLink, RouterLinkActive } from '@angular/router';

import {
  AvatarComponent,
  BreadcrumbRouterComponent,
  ContainerComponent,
  DropdownComponent,
  DropdownDividerDirective,
  DropdownHeaderDirective,
  DropdownItemDirective,
  DropdownMenuDirective,
  DropdownToggleDirective,
  HeaderComponent,
  HeaderNavComponent,
  HeaderTogglerDirective,
  NavItemComponent,
  SidebarToggleDirective,
} from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { AuthService } from '../../../core/auth.service';

interface OrgOption {
  id: string;
  name: string;
}

@Component({
  selector: 'app-default-header',
  templateUrl: './default-header.component.html',
  imports: [
    NgTemplateOutlet, NgIf,
    ContainerComponent, HeaderTogglerDirective, SidebarToggleDirective, IconDirective,
    HeaderNavComponent, NavItemComponent, RouterLink, RouterLinkActive,
    BreadcrumbRouterComponent, DropdownComponent, DropdownToggleDirective, AvatarComponent,
    DropdownMenuDirective, DropdownHeaderDirective, DropdownItemDirective, DropdownDividerDirective,
  ],
})
export class DefaultHeaderComponent extends HeaderComponent {
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);

  // Signals re-exportados del AuthService (para que el template los lea).
  readonly user = this.auth.user;
  readonly activeRole = this.auth.activeRole;
  readonly activeOrgId = this.auth.activeOrgId;
  readonly isImpersonating = this.auth.isImpersonating;

  // HU-41.1: super_admin puede cambiar el org activo. La lista de orgs
  // disponibles se carga on-demand desde /api/v1/organizations (filtra
  // por super_admin en backend). Por ahora, placeholder vacío hasta HU-41.9.
  readonly availableOrgs = signal<OrgOption[]>([]);

  readonly isSuperAdmin = computed(() => this.activeRole()?.slug === 'super_admin');

  readonly userInitials = computed(() => {
    const name = this.user()?.name ?? '';
    return name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map(p => (p[0] ?? '').toUpperCase())
      .join('');
  });

  readonly sidebarId = input('sidebar1');

  onOrgChange(orgId: string): void {
    if (!orgId) return;
    this.auth.setActiveOrg(orgId);
  }

  logout(): void {
    this.auth.logout().subscribe({
      complete: () => this.router.navigate(['/login']),
      error: () => this.router.navigate(['/login']),
    });
  }
}
