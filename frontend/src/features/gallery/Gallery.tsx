/**
 * The image gallery.
 *
 * Preserves the InvokeAI gallery experience: a dense thumbnail grid that
 * can be filtered, with selection driving the preview and comparison
 * views.
 *
 * The thumbnails are served by PocketBase's native file route on the
 * same origin as the UI, so no separate server is involved.
 */

import { useMemo, useState } from 'react';

import type { Image } from '../../invoke-adapter/types';

interface GalleryProps {
  images: Image[];
  selectedId: string;
  onSelect: (imageId: string) => void;
  onDelete: (imageId: string) => void;
  onExport: (imageId: string) => void;
}

/**
 * The gallery panel.
 */
export function Gallery({ images, selectedId, onSelect, onDelete, onExport }: GalleryProps) {
  const [filter, setFilter] = useState('');

  const visible = useMemo(() => {
    const needle = filter.trim().toLowerCase();
    if (!needle) {
      return images;
    }
    return images.filter((image) => image.prompt.toLowerCase().includes(needle));
  }, [images, filter]);

  return (
    <section className="panel">
      <div className="panel-header">
        <span>Gallery</span>
        <span className="text-dim">{visible.length}</span>
      </div>
      <div className="panel-body">
        <input
          type="search"
          placeholder="Filter by prompt…"
          value={filter}
          onChange={(event) => setFilter(event.target.value)}
        />

        {visible.length === 0 ? (
          <div className="empty-state">Generated images appear here.</div>
        ) : (
          <div className="gallery-grid">
            {visible.map((image) => (
              <div key={image.id} className="gallery-tile-wrap">
                <button
                  type="button"
                  className={`gallery-tile ${image.id === selectedId ? 'gallery-tile-selected' : ''}`}
                  title={image.prompt}
                  onClick={() => onSelect(image.id)}
                >
                  <img src={image.thumbnail_url || image.url} alt={image.prompt || 'Generated image'} />
                </button>
                <div className="row" style={{ gap: 4, marginTop: 4 }}>
                  <button
                    type="button"
                    className="btn btn-sm btn-danger"
                    style={{ flex: 1 }}
                    onClick={() => onDelete(image.id)}
                    aria-label="Delete image"
                  >
                    Delete
                  </button>
                  <button
                    type="button"
                    className="btn btn-sm"
                    style={{ flex: 1 }}
                    onClick={() => onExport(image.id)}
                    aria-label="Export image"
                  >
                    Save
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
