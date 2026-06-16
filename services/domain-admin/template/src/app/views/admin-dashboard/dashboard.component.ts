import { Component, inject, signal, OnInit, computed } from '@angular/core';
import { CommonModule, DecimalPipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { RouterLink } from '@angular/router';

import {
  CardComponent, CardBodyComponent, CardHeaderComponent,
  RowComponent, ColComponent,
  BadgeComponent,
  SpinnerComponent,
  AlertComponent,
} from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { AuthService } from '../../core/auth.service';

// HU-41.2: vista home del admin dashboard. Consume GET /api/v1/admin/org-overview
// y renderiza stat cards + top users + recent activity.

interface OrgOverview {
  org_id: string;
  stats: {
    members_active: number;
    agents: number;
    runs_24h: number;
    tokens_this_month: number;
    cost_this_month_usd: number;
  };
  top_users_this_month: TopUser[];
  recent_activity: RecentActivity[];
  system_health?: {
    api: string;
    database: string;
    llm_provider: string;
  };
}

interface TopUser {
  user_id: string;
  name: string;
  email: string;
  prompts: number;
  tokens_in: number;
  tokens_out: number;
  cost_usd: number;
}

interface RecentActivity {
  actor: string;
  action: string;
  target: string;
  at: string;
}

@Component({
  selector: 'app-admin-dashboard',
  templateUrl: './dashboard.component.html',
  imports: [
    CommonModule, DecimalPipe, RouterLink,
    CardComponent, CardBodyComponent, CardHeaderComponent,
    RowComponent, ColComponent,
    BadgeComponent, SpinnerComponent, AlertComponent, IconDirective,
  ],
})
export class AdminDashboardComponent implements OnInit {
  private readonly http = inject(HttpClient);
  private readonly auth = inject(AuthService);

  readonly overview = signal<OrgOverview | null>(null);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);
  readonly isSuperAdmin = computed(() => this.auth.activeRole()?.slug === 'super_admin');

  ngOnInit() {
    this.load();
  }

  load() {
    this.loading.set(true);
    this.error.set(null);
    this.http.get<OrgOverview>(`${apiBase()}/api/v1/admin/org-overview`).subscribe({
      next: res => {
        this.overview.set(res);
        this.loading.set(false);
      },
      error: err => {
        this.error.set(err?.error?.error?.message || 'No se pudo cargar el overview');
        this.loading.set(false);
      },
    });
  }

  // Para "tiempo relativo" en recent activity
  relativeTime(iso: string): string {
    const then = new Date(iso).getTime();
    const now = Date.now();
    const sec = Math.max(0, Math.floor((now - then) / 1000));
    if (sec < 60) return `hace ${sec}s`;
    const min = Math.floor(sec / 60);
    if (min < 60) return `hace ${min}m`;
    const hr = Math.floor(min / 60);
    if (hr < 24) return `hace ${hr}h`;
    const days = Math.floor(hr / 24);
    if (days < 30) return `hace ${days}d`;
    return new Date(iso).toLocaleDateString();
  }
}
