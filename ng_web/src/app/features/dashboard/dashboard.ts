import { Component, signal } from '@angular/core';
import { RouterLink } from '@angular/router';

@Component({
  selector: 'app-dashboard',
  imports: [RouterLink],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.scss',
})
export class Dashboard {
  protected readonly stats = signal({
    totalTemplates: 0,
  });

  protected readonly quickActions = [
    { label: 'Nouveau template', icon: '📝', route: '/templates/new', color: 'success' },
    { label: 'Liste des templates', icon: '📋', route: '/templates', color: 'primary' },
  ];
}
