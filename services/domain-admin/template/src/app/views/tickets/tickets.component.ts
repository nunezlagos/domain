import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { HttpClient } from '@angular/common/http';

import {
  CardComponent, CardBodyComponent, CardHeaderComponent,
  TableDirective, BadgeComponent, ButtonDirective,
  AlertComponent, FormControlDirective, FormSelectDirective,
  RowComponent, ColComponent, ModalComponent, ModalHeaderComponent,
  ModalBodyComponent, ModalFooterComponent, ModalTitleDirective,
  ButtonCloseDirective,
} from '@coreui/angular';

import { apiBase } from '../../core/runtime-config';
import { AuthService } from '../../core/auth.service';

interface Ticket {
  id: string;
  key: string;
  display_key: string;
  title: string;
  status: string;
  priority: string;
  issue_type: string;
  assignee_id?: string | null;
  locked_by?: string | null;
  locked_until?: string | null;
  updated_at: string;
}

interface User {
  id: string;
  email: string;
  name: string;
}

@Component({
  selector: 'app-tickets',
  templateUrl: './tickets.component.html',
  imports: [
    CommonModule, FormsModule,
    CardComponent, CardBodyComponent, CardHeaderComponent,
    TableDirective, BadgeComponent, ButtonDirective,
    AlertComponent, FormControlDirective, FormSelectDirective,
    RowComponent, ColComponent,
    ModalComponent, ModalHeaderComponent, ModalBodyComponent,
    ModalFooterComponent, ModalTitleDirective, ButtonCloseDirective,
  ],
})
export class TicketsComponent implements OnInit {
  private http = inject(HttpClient);
  auth = inject(AuthService);

  tickets = signal<Ticket[]>([]);
  loading = signal(false);
  error = signal<string | null>(null);
  filterStatus = signal('');

  // Reassign modal state
  reassignTarget = signal<Ticket | null>(null);
  reassignTo = signal<string>('');
  users = signal<User[]>([]);

  ngOnInit() {
    this.load();
  }

  load() {
    this.loading.set(true);
    this.error.set(null);
    const status = this.filterStatus();
    const url = `${apiBase()}/api/v1/tickets` + (status ? `?status=${status}` : '');
    this.http.get<{ data: { items: Ticket[] } }>(url).subscribe({
      next: res => {
        this.tickets.set(res?.data?.items || []);
        this.loading.set(false);
      },
      error: err => {
        this.error.set(err?.error?.error?.message || 'Error cargando tickets');
        this.loading.set(false);
      },
    });
  }

  statusColor(s: string): string {
    return ({
      'backlog': 'secondary',
      'todo': 'info',
      'in_progress': 'primary',
      'in_review': 'warning',
      'blocked': 'danger',
      'done': 'success',
      'cancelled': 'dark',
    } as Record<string, string>)[s] || 'secondary';
  }

  priorityColor(p: string): string {
    return ({
      'trivial': 'secondary',
      'low': 'info',
      'medium': 'primary',
      'high': 'warning',
      'critical': 'danger',
    } as Record<string, string>)[p] || 'secondary';
  }

  isLocked(t: Ticket): boolean {
    if (!t.locked_by || !t.locked_until) return false;
    return new Date(t.locked_until) > new Date();
  }

  openReassign(t: Ticket) {
    this.reassignTarget.set(t);
    this.reassignTo.set(t.assignee_id || '');
    if (this.users().length === 0) {
      // lazy load la lista de users de la org
      this.http.get<{ data: { items: User[] } }>(`${apiBase()}/api/v1/users`).subscribe({
        next: res => this.users.set(res?.data?.items || []),
        error: () => this.users.set([]),
      });
    }
  }

  closeReassign() {
    this.reassignTarget.set(null);
  }

  submitReassign() {
    const t = this.reassignTarget();
    if (!t) return;
    const body = { assignee_id: this.reassignTo() || null };
    this.http.patch(`${apiBase()}/api/v1/tickets/${t.id}`, body).subscribe({
      next: () => {
        this.closeReassign();
        this.load();
      },
      error: err => {
        this.error.set(err?.error?.error?.message || 'No se pudo reasignar');
      },
    });
  }
}
