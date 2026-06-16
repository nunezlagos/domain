import { Component, inject, signal, OnInit, computed } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { HttpClient } from '@angular/common/http';

import {
  CardComponent, CardBodyComponent, CardHeaderComponent,
  TableDirective, BadgeComponent, ButtonDirective,
  AlertComponent, FormControlDirective, FormSelectDirective,
  RowComponent, ColComponent,
  ModalComponent, ModalHeaderComponent, ModalBodyComponent, ModalFooterComponent,
  ModalTitleDirective, ButtonCloseDirective,
  SpinnerComponent,
} from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { AuthService } from '../../core/auth.service';

// HU-41.3: members management. Lista + invitar + crear con API key + cambiar rol
// + ver API keys. Las acciones destructivas (revocar) se simulan via soft-delete
// (UPDATE users SET deleted_at) — la HU-41.6 de "usage-by-user" usa el mismo patrón.

interface Member {
  user_id: string;
  email: string;
  name: string;
  role: string;
  joined_at: string;
  last_active: string | null;
}

interface Invitation {
  id: string;
  email: string;
  role: string;
  invited_at: string;
  expires_at: string;
}

interface Role {
  slug: string;
  name: string;
  permissions: string[];
}

interface ApiKey {
  id: string;
  prefix: string;
  name: string;
  created_at: string;
  last_used_at: string | null;
  expires_at: string | null;
}

interface CreateMemberResponse {
  user: { id: string; email: string; name: string };
  api_key: { plaintext: string; prefix: string };
}

@Component({
  selector: 'app-admin-members',
  templateUrl: './members.component.html',
  imports: [
    CommonModule, FormsModule,
    CardComponent, CardBodyComponent, CardHeaderComponent,
    TableDirective, BadgeComponent, ButtonDirective,
    AlertComponent, FormControlDirective, FormSelectDirective,
    RowComponent, ColComponent,
    ModalComponent, ModalHeaderComponent, ModalBodyComponent, ModalFooterComponent,
    ModalTitleDirective, ButtonCloseDirective,
    SpinnerComponent, IconDirective,
  ],
})
export class AdminMembersComponent implements OnInit {
  private readonly http = inject(HttpClient);
  private readonly auth = inject(AuthService);

  // Data
  readonly members = signal<Member[]>([]);
  readonly invitations = signal<Invitation[]>([]);
  readonly roles = signal<Role[]>([]);
  readonly orgId = computed(() => this.auth.user()?.organization_id);

  // UI state
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);
  readonly successMsg = signal<string | null>(null);

  // Modals
  readonly showInviteModal = signal(false);
  readonly showCreateModal = signal(false);
  readonly showApiKeysModal = signal(false);
  readonly showRoleModal = signal(false);
  readonly createdApiKey = signal<string | null>(null);

  // Form state
  readonly inviteEmail = signal('');
  readonly inviteName = signal('');
  readonly inviteRole = signal<string>('member');
  readonly createEmail = signal('');
  readonly createName = signal('');
  readonly createRole = signal<string>('member');
  readonly filter = signal('');

  // Selection
  readonly selectedMember = signal<Member | null>(null);
  readonly selectedMemberKeys = signal<ApiKey[]>([]);
  readonly newRole = signal<string>('');

  // Per-user usage (from HU-41.6 endpoint, optional)
  readonly usageByUser = signal<Record<string, number>>({});

  readonly filteredMembers = computed(() => {
    const f = this.filter().toLowerCase().trim();
    if (!f) return this.members();
    return this.members().filter(m =>
      m.email.toLowerCase().includes(f) ||
      m.name.toLowerCase().includes(f) ||
      m.role.toLowerCase().includes(f)
    );
  });

  ngOnInit() {
    this.load();
  }

  load() {
    const orgID = this.orgId();
    if (!orgID) {
      this.error.set('No se encontró la org del usuario');
      this.loading.set(false);
      return;
    }
    this.loading.set(true);
    this.error.set(null);

    // Load members, invitations y roles en paralelo
    Promise.all([
      this.http.get<Member[]>(`${apiBase()}/api/v1/organizations/${orgID}/members`).toPromise(),
      this.http.get<Invitation[]>(`${apiBase()}/api/v1/organizations/${orgID}/invitations`).toPromise().catch(() => []),
      this.http.get<Role[]>(`${apiBase()}/api/v1/organizations/${orgID}/roles`).toPromise().catch(() => []),
    ]).then(([members, invitations, roles]) => {
      this.members.set(members ?? []);
      this.invitations.set(invitations ?? []);
      this.roles.set(roles ?? []);
      this.loading.set(false);
    }).catch(err => {
      this.error.set(err?.error?.error?.message || 'No se pudieron cargar los miembros');
      this.loading.set(false);
    });
  }

  openInvite() {
    this.inviteEmail.set('');
    this.inviteName.set('');
    this.inviteRole.set(this.roles()[0]?.slug ?? 'member');
    this.showInviteModal.set(true);
  }

  submitInvite() {
    const orgID = this.orgId();
    if (!orgID) return;
    const email = this.inviteEmail().trim();
    const role = this.inviteRole();
    if (!email) {
      this.error.set('Email requerido');
      return;
    }
    this.error.set(null);
    this.http.post(`${apiBase()}/api/v1/organizations/${orgID}/invitations`, {
      email, role,
    }).subscribe({
      next: () => {
        this.showInviteModal.set(false);
        this.successMsg.set(`Invitación enviada a ${email}`);
        this.load();
      },
      error: err => {
        this.error.set(err?.error?.error?.message || 'No se pudo invitar');
      },
    });
  }

  openCreate() {
    this.createEmail.set('');
    this.createName.set('');
    this.createRole.set(this.roles()[0]?.slug ?? 'member');
    this.createdApiKey.set(null);
    this.showCreateModal.set(true);
  }

  submitCreate() {
    const orgID = this.orgId();
    if (!orgID) return;
    const email = this.createEmail().trim();
    const name = this.createName().trim();
    const role = this.createRole();
    if (!email || !name) {
      this.error.set('Email y nombre requeridos');
      return;
    }
    this.error.set(null);
    this.http.post<CreateMemberResponse>(`${apiBase()}/api/v1/organizations/${orgID}/members`, {
      email, name, role,
    }).subscribe({
      next: res => {
        this.createdApiKey.set(res.api_key.plaintext);
        this.load();
      },
      error: err => {
        this.error.set(err?.error?.error?.message || 'No se pudo crear el miembro');
      },
    });
  }

  closeCreateModal() {
    this.showCreateModal.set(false);
    this.createdApiKey.set(null);
  }

  openRoleChange(m: Member) {
    this.selectedMember.set(m);
    this.newRole.set(m.role);
    this.showRoleModal.set(true);
  }

  submitRoleChange() {
    const m = this.selectedMember();
    const orgID = this.orgId();
    if (!m || !orgID) return;
    const role = this.newRole();
    this.http.post(`${apiBase()}/api/v1/organizations/${orgID}/members/${m.user_id}/role`, { role })
      .subscribe({
        next: () => {
          this.showRoleModal.set(false);
          this.successMsg.set(`Rol de ${m.email} actualizado a ${role}`);
          this.load();
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudo cambiar el rol');
        },
      });
  }

  openApiKeys(m: Member) {
    this.selectedMember.set(m);
    this.selectedMemberKeys.set([]);
    this.showApiKeysModal.set(true);
    this.http.get<{ data: { items: ApiKey[] } } | ApiKey[]>(`${apiBase()}/api/v1/api-keys?user_id=${m.user_id}`)
      .subscribe({
        next: (res: any) => {
          const keys = Array.isArray(res) ? res : (res?.data?.items ?? []);
          this.selectedMemberKeys.set(keys);
        },
        error: () => {
          this.selectedMemberKeys.set([]);
        },
      });
  }

  revokeMember(m: Member) {
    if (!confirm(`¿Revocar acceso de ${m.email}?\n\nEsto invalidará sus API keys y sesiones.`)) return;
    const orgID = this.orgId();
    if (!orgID) return;
    this.http.delete(`${apiBase()}/api/v1/organizations/${orgID}/members/${m.user_id}`)
      .subscribe({
        next: () => {
          this.successMsg.set(`${m.email} fue removido de la org`);
          this.load();
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudo revocar (endpoint puede no existir; usar SQL en su lugar)');
        },
      });
  }

  roleBadgeColor(role: string): string {
    const map: Record<string, string> = {
      owner: 'danger',
      admin: 'primary',
      super_admin: 'warning',
      maintainer: 'info',
      member: 'secondary',
      viewer: 'light',
    };
    return map[role] ?? 'secondary';
  }

  userInitials(m: Member): string {
    const name = m.name || m.email;
    return name.split(/\s+/).filter(Boolean).slice(0, 2).map(p => (p[0] ?? '').toUpperCase()).join('') || '?';
  }

  copyToClipboard(text: string) {
    navigator.clipboard.writeText(text).then(() => {
      this.successMsg.set('Copiado al portapapeles');
    });
  }

  dismissMsg() {
    this.successMsg.set(null);
    this.error.set(null);
  }
}
