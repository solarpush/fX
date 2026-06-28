import { CommonModule } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { Component, inject, OnInit, signal } from '@angular/core';
import { environment } from '../../../../environments/environment';

@Component({
  selector: 'app-assets-manager',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './assets-manager.html',
  styleUrl: './assets-manager.scss',
})
export class AssetsManager implements OnInit {
  private readonly http = inject(HttpClient);

  protected readonly imagesList = signal<{name: string, url: string, size: number}[]>([]);
  protected readonly isUploadingImage = signal(false);

  ngOnInit(): void {
    this.loadImages();
  }

  protected loadImages() {
    this.http.get<any[]>(`${environment.apiUrl}/images`).subscribe({
      next: (imgs) => this.imagesList.set(imgs || []),
      error: (err) => console.error('Erreur chargement images', err)
    });
  }

  protected onImageSelected(event: any) {
    const file = event.target.files[0];
    if (!file) return;

    this.isUploadingImage.set(true);
    const formData = new FormData();
    formData.append('image', file);

    this.http.post(`${environment.apiUrl}/images`, formData).subscribe({
      next: () => {
        this.loadImages();
        this.isUploadingImage.set(false);
      },
      error: (err) => {
        console.error('Erreur upload image', err);
        alert('Erreur lors de l\'upload de l\'image (Format non supporté ou trop volumineuse ?)');
        this.isUploadingImage.set(false);
      }
    });
  }

  protected deleteImage(filename: string) {
    if (!confirm(`Supprimer l'image ${filename} ?`)) return;
    this.http.delete(`${environment.apiUrl}/images/${filename}`).subscribe({
      next: () => this.loadImages(),
      error: (err) => console.error('Erreur suppression image', err)
    });
  }

  protected copyImageUrl(url: string) {
    navigator.clipboard.writeText(url).then(() => {
      // Feedback
    });
  }
}
