import { CommonModule } from '@angular/common';
import { Component, OnDestroy, effect, inject, input, signal } from '@angular/core';
import { DomSanitizer, SafeResourceUrl } from '@angular/platform-browser';

@Component({
  selector: 'app-live-preview',
  imports: [CommonModule],
  templateUrl: './live-preview.html',
  styleUrl: './live-preview.scss',
})
export class LivePreview implements OnDestroy {
  private readonly sanitizer = inject(DomSanitizer);

  readonly previewBlob = input<Blob | null>(null);
  readonly isLoading = input(false);

  protected readonly previewUrl = signal<SafeResourceUrl | null>(null);
  private activeObjectUrl: string | null = null;

  constructor() {
    effect((onCleanup) => {
      const blob = this.previewBlob();
      this.revokeCurrentUrl();

      if (blob) {
        const url = URL.createObjectURL(blob);
        this.activeObjectUrl = url;
        this.previewUrl.set(this.sanitizer.bypassSecurityTrustResourceUrl(url));
      } else {
        this.previewUrl.set(null);
      }

      onCleanup(() => {
        this.revokeCurrentUrl();
      });
    });
  }

  ngOnDestroy(): void {
    this.revokeCurrentUrl();
  }

  private revokeCurrentUrl(): void {
    if (this.activeObjectUrl) {
      URL.revokeObjectURL(this.activeObjectUrl);
      this.activeObjectUrl = null;
    }
  }
}
