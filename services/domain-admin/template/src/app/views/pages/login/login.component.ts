import { Component, inject, signal, OnInit, computed } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router, ActivatedRoute } from '@angular/router';
import { CommonModule } from '@angular/common';
import { IconDirective } from '@coreui/icons-angular';
import {
  ButtonDirective,
  CardBodyComponent,
  ColComponent,
  ContainerComponent,
  FormControlDirective,
  InputGroupComponent,
  InputGroupTextDirective,
  RowComponent,
  AlertComponent,
  SpinnerComponent,
} from '@coreui/angular';

import { AuthService, Role } from '../../../core/auth.service';

// HU-UX-LOGIN: split layout 60/40 responsive, design system moderno.
// - Mobile (<md): full-screen, solo el form, branding como header
// - Desktop (md+): 60% form (izquierda) + 40% branding (derecha, oculto en mobile)
// - Step 1: login con email + password (autocomplete correcto, password toggle, remember email)
// - Step 2: role selection con cards (no list-group) + permission chips
// - Dark mode compatible (CoreUI theme variables)
@Component({
  selector: 'app-login',
  templateUrl: './login.component.html',
  styleUrls: ['./login.component.scss'],
  imports: [
    CommonModule, FormsModule,
    ContainerComponent, RowComponent, ColComponent,
    FormControlDirective, InputGroupComponent, InputGroupTextDirective,
    IconDirective, ButtonDirective, AlertComponent, SpinnerComponent,
    CardBodyComponent,
  ],
})
export class LoginComponent implements OnInit {
  private auth = inject(AuthService);
  private router = inject(Router);
  private route = inject(ActivatedRoute);

  // Form state
  email = '';
  password = '';
  loading = signal(false);
  error = signal<string | null>(null);
  showPassword = signal(false);

  // Role selection state
  tempToken = signal<string | null>(null);
  roles = signal<Role[]>([]);
  selectedRole = signal<string>('');

  // Computed: skip role selection if user has exactly 1 role
  private readonly singleRole = computed(() =>
    this.roles().length === 1 ? this.roles()[0] : null,
  );

  private readonly REMEMBERED_EMAIL_KEY = 'domain.remembered_email';

  ngOnInit() {
    // HU-UX-LOGIN: recordar email entre sesiones (NO password).
    const remembered = localStorage.getItem(this.REMEMBERED_EMAIL_KEY);
    if (remembered) this.email = remembered;
  }

  togglePassword(): void {
    this.showPassword.update(v => !v);
  }

  submitLogin(): void {
    if (!this.email || !this.password) return;
    this.error.set(null);
    this.loading.set(true);
    this.auth.login(this.email, this.password).subscribe({
      next: res => {
        this.loading.set(false);
        this.tempToken.set(res.temp_token);
        this.roles.set(res.roles);
        // Persistir email (recordar entre sesiones)
        localStorage.setItem(this.REMEMBERED_EMAIL_KEY, this.email);
        // Auto-skip si solo hay 1 rol
        if (res.roles.length === 1) {
          this.selectedRole.set(res.roles[0].slug);
          this.submitRole();
        }
      },
      error: err => {
        this.loading.set(false);
        this.error.set(err?.error?.error?.message || err?.error?.error || 'Credenciales inválidas');
      },
    });
  }

  submitRole(): void {
    const tok = this.tempToken();
    const slug = this.selectedRole();
    if (!tok || !slug) return;
    this.error.set(null);
    this.loading.set(true);
    this.auth.selectRole(tok, slug).subscribe({
      next: () => {
        this.loading.set(false);
        const returnUrl = this.route.snapshot.queryParamMap.get('returnUrl') || '/admin/dashboard';
        this.router.navigateByUrl(returnUrl);
      },
      error: err => {
        this.loading.set(false);
        this.error.set(err?.error?.error?.message || err?.error?.error || 'No se pudo seleccionar el rol');
      },
    });
  }

  back(): void {
    this.tempToken.set(null);
    this.roles.set([]);
    this.selectedRole.set('');
    this.error.set(null);
  }
}
