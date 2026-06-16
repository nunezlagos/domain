import { NgTemplateOutlet, NgIf } from '@angular/common';
import { Component, computed, inject, input, signal } from '@angular/core';
import { Router, RouterLink, RouterLinkActive } from '@angular/router';

import {
  AvatarComponent,
  BreadcrumbRouterComponent,
  ColorModeService,
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

interface ThemeMode {
  name: string;
  text: string;
  icon: string;
}

@Component({
  selector: 'app-default-header',
  templateUrl: './default-header.component.html',
  imports: [
    NgTemplateOutlet, NgIf,
    ContainerComponent, HeaderTogglerDirective, SidebarToggleDirective, IconDirective,
    HeaderComponent, HeaderNavComponent, NavItemComponent, RouterLink, RouterLinkActive,
    BreadcrumbRouterComponent, DropdownComponent, DropdownToggleDirective, AvatarComponent,
    DropdownMenuDirective, DropdownHeaderDirective, DropdownItemDirective, DropdownDividerDirective,
  ],
})
export class DefaultHeaderComponent extends HeaderComponent {
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);
  private readonly colorModeService = inject(ColorModeService);

  readonly user = this.auth.user;
  readonly activeRole = this.auth.activeRole;
  readonly activeOrgId = this.auth.activeOrgId;
  readonly isImpersonating = this.auth.isImpersonating;
  readonly colorMode = this.colorModeService.colorMode;

  readonly availableOrgs = signal<OrgOption[]>([]);

  readonly isSuperAdmin = computed(() => this.activeRole()?.slug === 'super_admin');

  readonly userInitials = computed(() => {
    const name = this.user()?.name ?? '';
    return name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map(p => (p[0] ?? '').toUpperCase())
      .join('') || '?';
  });

  readonly colorModes: ThemeMode[] = [
    { name: 'light', text: 'Light', icon: 'cilSun' },
    { name: 'dark', text: 'Dark', icon: 'cilMoon' },
    { name: 'auto', text: 'Auto', icon: 'cilContrast' },
  ];

  readonly themeIcon = computed(() => {
    const mode = this.colorMode();
    return this.colorModes.find(m => m.name === mode)?.icon ?? 'cilSun';
  });

  readonly sidebarId = input('sidebar1');

  setTheme(mode: string): void {
    this.colorModeService.colorMode.set(mode as 'light' | 'dark' | 'auto');
    localStorage.setItem('domain.theme', mode);
  }

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
