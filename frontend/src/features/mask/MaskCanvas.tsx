/**
 * The mask drawing canvas.
 *
 * Preserves the InvokeAI inpainting/outpainting mask experience: the
 * user paints over the region they want to affect.
 *
 * MiniMax Image-01 has no alpha mask input, so the painted mask is
 * applied locally to the generated image by the Go backend. The UI
 * states this plainly rather than implying model-side inpainting.
 */

import { useCallback, useEffect, useRef, useState } from 'react';

interface MaskCanvasProps {
  /** The image the mask is drawn over, if any. */
  backgroundUrl: string;
  /** Brush radius in canvas pixels. */
  brushSize: number;
  /** Called when the mask changes, with a PNG blob of the mask. */
  onMaskChange: (blob: Blob | null) => void;
  /** Called when the mask is cleared. */
  onClear: () => void;
}

/**
 * The mask drawing canvas.
 */
export function MaskCanvas({ backgroundUrl, brushSize, onMaskChange, onClear }: MaskCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const backgroundRef = useRef<HTMLImageElement | null>(null);
  const drawingRef = useRef(false);
  const lastPointRef = useRef<{ x: number; y: number } | null>(null);
  const [size, setSize] = useState({ width: 512, height: 512 });

  // Draw the background image once it loads, then restore the mask.
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }
    const context = canvas.getContext('2d');
    if (!context) {
      return;
    }

    if (!backgroundUrl) {
      context.clearRect(0, 0, canvas.width, canvas.height);
      backgroundRef.current = null;
      return;
    }

    const image = new Image();
    image.onload = () => {
      backgroundRef.current = image;
      canvas.width = image.naturalWidth;
      canvas.height = image.naturalHeight;
      setSize({ width: image.naturalWidth, height: image.naturalHeight });

      const ctx = canvas.getContext('2d');
      if (!ctx) {
        return;
      }
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      ctx.drawImage(image, 0, 0);
    };
    image.src = backgroundUrl;
  }, [backgroundUrl]);

  const canvasPoint = useCallback((event: React.PointerEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return null;
    }
    const rect = canvas.getBoundingClientRect();
    const scaleX = canvas.width / rect.width;
    const scaleY = canvas.height / rect.height;
    return {
      x: (event.clientX - rect.left) * scaleX,
      y: (event.clientY - rect.top) * scaleY,
    };
  }, []);

  const paint = useCallback(
    (from: { x: number; y: number }, to: { x: number; y: number }) => {
      const canvas = canvasRef.current;
      if (!canvas) {
        return;
      }
      const context = canvas.getContext('2d');
      if (!context) {
        return;
      }
      context.strokeStyle = 'rgba(255, 255, 255, 0.85)';
      context.lineWidth = brushSize;
      context.lineCap = 'round';
      context.lineJoin = 'round';
      context.beginPath();
      context.moveTo(from.x, from.y);
      context.lineTo(to.x, to.y);
      context.stroke();
    },
    [brushSize],
  );

  const emitMask = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }

    // Build a dedicated mask canvas: the drawn strokes become the alpha
    // channel, so the Go backend can composite with it directly.
    const mask = document.createElement('canvas');
    mask.width = canvas.width;
    mask.height = canvas.height;
    const maskContext = mask.getContext('2d');
    if (!maskContext) {
      return;
    }

    maskContext.fillStyle = '#000000';
    maskContext.fillRect(0, 0, mask.width, mask.height);
    maskContext.drawImage(canvas, 0, 0);

    // Convert the luminance into alpha.
    const pixels = maskContext.getImageData(0, 0, mask.width, mask.height);
    const data = pixels.data;
    for (let i = 0; i < data.length; i += 4) {
      const luminance = (data[i] + data[i + 1] + data[i + 2]) / 3;
      // The background image is dark, so anything painted is bright.
      const painted = luminance > 128 ? 255 : 0;
      data[i] = 255;
      data[i + 1] = 255;
      data[i + 2] = 255;
      data[i + 3] = painted;
    }
    maskContext.putImageData(pixels, 0, 0);

    mask.toBlob((blob) => {
      onMaskChange(blob);
    }, 'image/png');
  }, [onMaskChange]);

  const handlePointerDown = useCallback(
    (event: React.PointerEvent<HTMLCanvasElement>) => {
      const point = canvasPoint(event);
      if (!point) {
        return;
      }
      drawingRef.current = true;
      lastPointRef.current = point;
      event.currentTarget.setPointerCapture(event.pointerId);
      paint(point, point);
    },
    [canvasPoint, paint],
  );

  const handlePointerMove = useCallback(
    (event: React.PointerEvent<HTMLCanvasElement>) => {
      if (!drawingRef.current) {
        return;
      }
      const point = canvasPoint(event);
      const last = lastPointRef.current;
      if (!point || !last) {
        return;
      }
      paint(last, point);
      lastPointRef.current = point;
    },
    [canvasPoint, paint],
  );

  const handlePointerUp = useCallback(
    (event: React.PointerEvent<HTMLCanvasElement>) => {
      if (!drawingRef.current) {
        return;
      }
      drawingRef.current = false;
      lastPointRef.current = null;
      event.currentTarget.releasePointerCapture(event.pointerId);
      emitMask();
    },
    [emitMask],
  );

  const handleClear = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }
    const context = canvas.getContext('2d');
    if (!context) {
      return;
    }
    context.clearRect(0, 0, canvas.width, canvas.height);
    const background = backgroundRef.current;
    if (background) {
      context.drawImage(background, 0, 0);
    }
    onClear();
  }, [onClear]);

  return (
    <div className="mask-tools">
      <div className="mask-stage">
        <canvas
          ref={canvasRef}
          width={size.width}
          height={size.height}
          className="mask-overlay"
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          onPointerCancel={handlePointerUp}
        />
      </div>
      <div className="row">
        <button type="button" className="btn btn-sm" onClick={handleClear}>
          Clear mask
        </button>
      </div>
    </div>
  );
}
