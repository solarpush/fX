import { Component, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { AuthService } from '../../../core/auth';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './login.html',
  styleUrl: './login.scss'
})
export class Login {
  private auth = inject(AuthService);
  private router = inject(Router);

  protected password = signal('');
  protected isLoading = signal(false);
  protected error = signal('');

  protected onSubmit() {
    if (!this.password()) return;
    
    this.isLoading.set(true);
    this.error.set('');

    this.auth.login(this.password()).subscribe({
      next: () => {
        this.router.navigate(['/dashboard']);
      },
      error: () => {
        this.error.set('Mot de passe incorrect');
        this.isLoading.set(false);
      }
    });
  }
}
