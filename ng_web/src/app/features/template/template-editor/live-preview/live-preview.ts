import { CommonModule } from '@angular/common';
import { Component, effect, input, signal } from '@angular/core';
import { DomSanitizer, SafeResourceUrl } from '@angular/platform-browser';

@Component({
  selector: 'app-live-preview',
  imports: [CommonModule],
  templateUrl: './live-preview.html',
  styleUrl: './live-preview.scss',
})
export class LivePreview {
  readonly previewBlob = input<Blob | null>(null);
  readonly isLoading = input(false);

  protected readonly previewUrl = signal<SafeResourceUrl | null>(null);

  constructor(private sanitizer: DomSanitizer) {
    effect(() => {
      const blob = this.previewBlob();
      if (blob) {
        const url = URL.createObjectURL(blob);
        this.previewUrl.set(this.sanitizer.bypassSecurityTrustResourceUrl(url));
      } else {
        this.previewUrl.set(null);
      }
    });
  }
}
