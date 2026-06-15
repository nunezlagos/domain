import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router, ActivatedRoute } from '@angular/router';
import { CommonModule } from '@angular/common';
import { IconDirective } from '@coreui/icons-angular';
import {
  ButtonDirective,
  CardBodyComponent,
  CardComponent,
  CardGroupComponent,
  ColComponent,
  ContainerComponent,
  FormControlDirective,
  FormDirective,
  InputGroupComponent,
  InputGroupTextDirective,
  RowComponent,
  AlertComponent,
} from '@coreui/angular';

import { AuthService, Role } from '../../../core/auth.service';

@Component({
  selector: 'app-login',
  templateUrl: './login.component.html',
  imports: [
    CommonModule, FormsModule,
    ContainerComponent, RowComponent, ColComponent,
    CardGroupComponent, CardComponent, CardBodyComponent,
    FormDirective, InputGroupComponent, InputGroupTextDirective,
    IconDirective, FormControlDirective, ButtonDirective,
    AlertComponent,
  ],
})
export class LoginComponent {
  private auth = inject(AuthService);
  private router = inject(Router);
  private route = inject(ActivatedRoute);

  email = '';
  password = '';
  tempToken = signal<string | null>(null);
  roles = signal<Role[]>([]);
  selectedRole = signal<string>('');
  loading = signal(false);
  error = signal<string | null>(null);

  submitLogin() {
    this.error.set(null);
    this.loading.set(true);
    this.auth.login(this.email, this.password).subscribe({
      next: res => {
        this.loading.set(false);
        this.tempToken.set(res.temp_token);
        this.roles.set(res.roles);
        if (res.roles.length === 1) {
          this.selectedRole.set(res.roles[0].slug);
          this.submitRole();
        }
      },
      error: err => {
        this.loading.set(false);
        this.error.set(err?.error?.error || 'Credenciales inválidas');
      },
    });
  }

  submitRole() {
    const tok = this.tempToken();
    const slug = this.selectedRole();
    if (!tok || !slug) return;
    this.error.set(null);
    this.loading.set(true);
    this.auth.selectRole(tok, slug).subscribe({
      next: () => {
        this.loading.set(false);
        const returnUrl = this.route.snapshot.queryParamMap.get('returnUrl') || '/dashboard';
        this.router.navigateByUrl(returnUrl);
      },
      error: err => {
        this.loading.set(false);
        this.error.set(err?.error?.error || 'No se pudo seleccionar el rol');
      },
    });
  }

  back() {
    this.tempToken.set(null);
    this.roles.set([]);
    this.selectedRole.set('');
    this.error.set(null);
  }
}
