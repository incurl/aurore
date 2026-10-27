/**
 * The Aurore application shell.
 *
 * The layout follows the InvokeAI workspace: a control rail on the left
 * (prompt, parameters, reference assets, presets), a canvas in the middle
 * (preview and comparison) and a gallery on the right, with the job
 * queue in the header area.
 *
 * All backend access goes through the Wails bindings; the frontend never
 * talks to PocketBase or MiniMax directly.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Events } from '@wailsio/runtime';

import './styles.css';

import {
  applyCapabilities,
  DEFAULT_FORM_STATE,
  toGenerationRequest,
  validateForm,
} from './invoke-adapter/parameters';
import type {
  Asset,
  AspectRatio,
  Capabilities,
  GenerationFormState,
  Image,
  Job,
  MiniMaxModel,
  Preset,
  Settings,
  StyleType,
} from './invoke-adapter/types';
import * as api from './wails-bindings';
import { applyZoom } from './ui/zoom';
import { AssetPanel } from './features/generation/AssetPanel';
import { ParameterPanel } from './features/generation/ParameterPanel';
import { PromptEditor } from './features/generation/PromptEditor';
import { Gallery } from './features/gallery/Gallery';
import { MaskCanvas } from './features/mask/MaskCanvas';
import { PresetPanel } from './features/presets/PresetPanel';
import { JobQueue } from './features/queue/JobQueue';
import { SettingsDialog } from './features/settings/SettingsDialog';
import { ConsolePanel } from './features/console/ConsolePanel';

interface Toast {
  message: string;
  kind: 'info' | 'error' | 'success';
}

/**
 * The application root.
 */
function App() {
  const queryClient = useQueryClient();

  const [form, setForm] = useState<GenerationFormState>(DEFAULT_FORM_STATE);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [selectedImageId, setSelectedImageId] = useState('');
  const [compareImageId, setCompareImageId] = useState('');
  const [toast, setToast] = useState<Toast | null>(null);
  const [maskBlob, setMaskBlob] = useState<Blob | null>(null);

  const notify = useCallback((message: string, kind: Toast['kind'] = 'info') => {
    setToast({ message, kind });
    window.setTimeout(() => setToast(null), 4000);
  }, []);

  // --- Data ---------------------------------------------------------

  const settingsQuery = useQuery({
    queryKey: ['settings'],
    queryFn: () => api.getSettings(),
  });

  const capabilitiesQuery = useQuery({
    queryKey: ['capabilities', form.model],
    queryFn: () => api.getCapabilities(form.model),
  });

  const jobsQuery = useQuery({
    queryKey: ['jobs'],
    queryFn: () => api.listJobs(50),
    // The Wails event pushes updates; this interval is the fallback.
    refetchInterval: 5000,
  });

  const imagesQuery = useQuery({
    queryKey: ['images'],
    queryFn: () => api.listImages('', 200),
    refetchInterval: 5000,
  });

  const assetsQuery = useQuery({
    queryKey: ['assets'],
    queryFn: () => api.listAssets('', 50),
  });

  const presetsQuery = useQuery({
    queryKey: ['presets'],
    queryFn: () => api.listPresets(),
  });

  const jobs: Job[] = useMemo(() => jobsQuery.data ?? [], [jobsQuery.data]);
  const images: Image[] = useMemo(() => imagesQuery.data ?? [], [imagesQuery.data]);
  const assets: Asset[] = useMemo(() => assetsQuery.data ?? [], [assetsQuery.data]);
  const presets: Preset[] = useMemo(() => presetsQuery.data ?? [], [presetsQuery.data]);

  // Seed the form from the saved defaults once they arrive.
  useEffect(() => {
    const settings = settingsQuery.data;
    if (!settings) {
      return;
    }
    setForm((current) => {
      if (current.prompt !== '' || current.width !== DEFAULT_FORM_STATE.width) {
        return current;
      }
      return {
        ...current,
        model: (settings.defaults.model || settings.model) as MiniMaxModel,
        aspectRatio: (settings.defaults.aspect_ratio || '1:1') as AspectRatio,
        width: settings.defaults.width || 1024,
        height: settings.defaults.height || 1024,
        batchSize: settings.defaults.batch_size || 1,
        seed: settings.defaults.seed || 0,
        promptOptimizer: settings.defaults.prompt_optimizer,
        watermark: settings.defaults.watermark,
        styleType: (settings.defaults.style_type || '') as StyleType | '',
        styleWeight: settings.defaults.style_weight || 0.8,
      };
    });
  }, [settingsQuery.data]);

  // Apply the stored interface zoom as soon as the settings arrive.
  //
  // The zoom is also previewed live in the settings dialog, so the
  // persisted value is only re-applied when it actually changes. That
  // stops a settings refetch from cancelling the live preview.
  const appliedZoomRef = useRef<number | null>(null);
  useEffect(() => {
    const settings = settingsQuery.data;
    if (!settings) {
      return;
    }
    const zoom = settings.ui?.zoom ?? 100;
    if (appliedZoomRef.current === zoom) {
      return;
    }
    appliedZoomRef.current = zoom;
    applyZoom(zoom);
  }, [settingsQuery.data]);

  // Apply the capability gate whenever the capabilities change.
  useEffect(() => {
    const capabilities = capabilitiesQuery.data;
    if (!capabilities) {
      return;
    }
    setForm((current) => {
      const next = applyCapabilities(current, capabilities);
      if (
        next.width === current.width &&
        next.height === current.height &&
        next.aspectRatio === current.aspectRatio &&
        next.styleType === current.styleType &&
        next.styleWeight === current.styleWeight
      ) {
        return current;
      }
      return next;
    });
  }, [capabilitiesQuery.data]);

  // Listen for job updates pushed from Go.
  useEffect(() => {
    const off = Events.On(api.EVENT_JOB_UPDATED, () => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
      void queryClient.invalidateQueries({ queryKey: ['images'] });
    });
    return () => {
      off();
    };
  }, [queryClient]);

  // --- Debug console -------------------------------------------------

  // The console entries are held in the frontend so that the panel
  // updates immediately. The backend keeps its own copy, which is read
  // once at startup.
  const [consoleEntries, setConsoleEntries] = useState<api.ConsoleEntry[]>([]);

  // Listen for console entries pushed from Go.
  useEffect(() => {
    const off = Events.On(api.EVENT_CONSOLE_ENTRY, (event) => {
      const entry = event.data;
      setConsoleEntries((current) => {
        const next = [...current, entry];
        // Keep the panel bounded, matching the backend limit.
        return next.length > 500 ? next.slice(next.length - 500) : next;
      });
    });
    return () => {
      off();
    };
  }, []);

  // Load the entries that were recorded before the window opened.
  useEffect(() => {
    let cancelled = false;
    void api
      .listConsoleEntries()
      .then((entries) => {
        if (!cancelled && entries) {
          setConsoleEntries(entries);
        }
      })
      .catch(() => {
        // The console is a debugging aid, so a failure to read it is
        // not worth reporting.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const clearConsole = useCallback(() => {
    setConsoleEntries([]);
    void api.clearConsoleEntries().catch(() => {
      // The panel is cleared locally regardless.
    });
  }, []);

  // --- Mutations ----------------------------------------------------

  const submitMutation = useMutation({
    mutationFn: async (request: ReturnType<typeof toGenerationRequest>) => {
      let referenceAssetId = request.reference_asset_id;
      let maskAssetId = request.mask_asset_id;

      // Upload a freshly drawn mask before submitting.
      if (maskBlob) {
        const bytes = new Uint8Array(await maskBlob.arrayBuffer());
        const uploaded = await api.uploadAsset('mask', bytes, 'mask.png');
        maskAssetId = uploaded.id;
        setMaskBlob(null);
      }

      return api.submitJob({ ...request, reference_asset_id: referenceAssetId, mask_asset_id: maskAssetId });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
      notify('Job submitted.', 'success');
    },
    onError: (error: unknown) => {
      notify(error instanceof Error ? error.message : String(error), 'error');
    },
  });

  const cancelMutation = useMutation({
    mutationFn: (jobId: string) => api.cancelJob(jobId),
    onError: (error: unknown) => {
      notify(error instanceof Error ? error.message : String(error), 'error');
    },
  });

  const retryMutation = useMutation({
    mutationFn: (jobId: string) => api.retryJob(jobId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: (error: unknown) => {
      notify(error instanceof Error ? error.message : String(error), 'error');
    },
  });

  const uploadMutation = useMutation({
    mutationFn: async ({ kind, file }: { kind: 'reference' | 'mask'; file: File }) => {
      const bytes = new Uint8Array(await file.arrayBuffer());
      return api.uploadAsset(kind, bytes, file.name);
    },
    onSuccess: (asset) => {
      void queryClient.invalidateQueries({ queryKey: ['assets'] });
      setForm((current) =>
        asset.kind === 'reference'
          ? { ...current, referenceAssetId: asset.id }
          : { ...current, maskAssetId: asset.id },
      );
    },
    onError: (error: unknown) => {
      notify(error instanceof Error ? error.message : String(error), 'error');
    },
  });

  const deleteImageMutation = useMutation({
    mutationFn: (imageId: string) => api.deleteImage(imageId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['images'] });
      setSelectedImageId('');
    },
    onError: (error: unknown) => {
      notify(error instanceof Error ? error.message : String(error), 'error');
    },
  });

  const deleteAssetMutation = useMutation({
    mutationFn: (assetId: string) => api.deleteAsset(assetId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['assets'] });
    },
    onError: (error: unknown) => {
      notify(error instanceof Error ? error.message : String(error), 'error');
    },
  });

  const savePresetMutation = useMutation({
    mutationFn: (name: string) =>
      api.savePreset({
        id: '',
        name,
        prompt: form.prompt,
        negative_prompt: form.negativePrompt,
        params: toGenerationRequest(form).params,
        is_default: false,
        created: '',
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['presets'] });
      notify('Preset saved.', 'success');
    },
    onError: (error: unknown) => {
      notify(error instanceof Error ? error.message : String(error), 'error');
    },
  });

  const deletePresetMutation = useMutation({
    mutationFn: (presetId: string) => api.deletePreset(presetId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['presets'] });
    },
    onError: (error: unknown) => {
      notify(error instanceof Error ? error.message : String(error), 'error');
    },
  });

  const exportMutation = useMutation({
    mutationFn: async (imageId: string) => {
      const bytes = await api.exportImage(imageId);
      const blob = new Blob([bytes as BlobPart], { type: 'image/png' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = `aurore-${imageId}.png`;
      anchor.click();
      URL.revokeObjectURL(url);
    },
    onError: (error: unknown) => {
      notify(error instanceof Error ? error.message : String(error), 'error');
    },
  });

  const credentialsMutation = useMutation({
    mutationFn: ({ apiKey, baseUrl, model }: { apiKey: string; baseUrl: string; model: string }) =>
      api.saveCredentials(apiKey, baseUrl, model),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['settings'] });
      notify('Credentials saved.', 'success');
    },
    onError: (error: unknown) => {
      notify(error instanceof Error ? error.message : String(error), 'error');
    },
  });

  const testConnectionMutation = useMutation({
    mutationFn: () => api.testCredentials(),
    onSuccess: () => notify('MiniMax credentials are valid.', 'success'),
    onError: (error: unknown) => {
      notify(error instanceof Error ? error.message : String(error), 'error');
    },
  });

  const uiMutation = useMutation({
    mutationFn: (ui: Settings['ui']) => api.saveUI(ui),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['settings'] });
      notify('Appearance saved.', 'success');
    },
    onError: (error: unknown) => {
      notify(error instanceof Error ? error.message : String(error), 'error');
    },
  });

  const defaultsMutation = useMutation({
    mutationFn: (defaults: Settings['defaults']) => api.saveDefaults(defaults),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['settings'] });
      notify('Defaults saved.', 'success');
    },
    onError: (error: unknown) => {
      notify(error instanceof Error ? error.message : String(error), 'error');
    },
  });

  // --- Derived state ------------------------------------------------

  const problems = validateForm(form);
  const capabilities: Capabilities = capabilitiesQuery.data ?? {
    model: form.model,
    supports_dimensions: form.model !== 'image-01-live',
    supports_style: form.model === 'image-01-live',
    supports_reference_image: true,
    supports_negative_prompt: false,
    supports_mask: false,
    aspect_ratios: ['1:1'],
    style_types: [],
    min_dimension: 512,
    max_dimension: 2048,
    max_batch_size: 9,
    max_prompt_length: 1500,
  };

  const selectedImage = images.find((image) => image.id === selectedImageId) ?? null;
  const compareImage = images.find((image) => image.id === compareImageId) ?? null;
  const hasApiKey = settingsQuery.data?.has_api_key ?? false;

  const handleSubmit = useCallback(() => {
    if (problems.length > 0) {
      notify(problems[0], 'error');
      return;
    }
    submitMutation.mutate(toGenerationRequest(form));
  }, [form, notify, problems, submitMutation]);

  const updateForm = useCallback((patch: Partial<GenerationFormState>) => {
    setForm((current) => ({ ...current, ...patch }));
  }, []);

  return (
    <div className="app">
      <header className="app-header">
        <span className="app-brand">
          Aurore
          <small>MiniMax Image-01</small>
        </span>
        <span className="app-header-spacer" />
        <button type="button" className="btn btn-sm" onClick={() => setSettingsOpen(true)}>
          Settings
        </button>
        <button
          type="button"
          className="btn btn-sm btn-primary"
          onClick={handleSubmit}
          disabled={submitMutation.isPending || problems.length > 0}
        >
          {submitMutation.isPending ? 'Submitting…' : 'Generate'}
        </button>
      </header>

      <aside className="app-rail">
        <PromptEditor
          prompt={form.prompt}
          negativePrompt={form.negativePrompt}
          useNegativePrompt={form.useNegativePrompt}
          onPromptChange={(value) => updateForm({ prompt: value })}
          onNegativePromptChange={(value) => updateForm({ negativePrompt: value })}
          onUseNegativePromptChange={(value) => updateForm({ useNegativePrompt: value })}
        />

        <ParameterPanel
          capabilities={capabilities}
          model={form.model}
          aspectRatio={form.aspectRatio}
          width={form.width}
          height={form.height}
          batchSize={form.batchSize}
          seed={form.seed}
          promptOptimizer={form.promptOptimizer}
          watermark={form.watermark}
          styleType={form.styleType}
          styleWeight={form.styleWeight}
          onModelChange={(value) => updateForm({ model: value })}
          onAspectRatioChange={(value) => updateForm({ aspectRatio: value })}
          onWidthChange={(value) => updateForm({ width: value })}
          onHeightChange={(value) => updateForm({ height: value })}
          onBatchSizeChange={(value) => updateForm({ batchSize: value })}
          onSeedChange={(value) => updateForm({ seed: value })}
          onPromptOptimizerChange={(value) => updateForm({ promptOptimizer: value })}
          onWatermarkChange={(value) => updateForm({ watermark: value })}
          onStyleTypeChange={(value) => updateForm({ styleType: value })}
          onStyleWeightChange={(value) => updateForm({ styleWeight: value })}
          onRandomiseSeed={() => updateForm({ seed: Math.floor(Math.random() * 2_147_483_647) })}
        />

        <AssetPanel
          assets={assets}
          referenceAssetId={form.referenceAssetId}
          maskAssetId={form.maskAssetId}
          onUpload={(kind, file) => uploadMutation.mutate({ kind, file })}
          onSelect={(kind, assetId) =>
            updateForm(kind === 'reference' ? { referenceAssetId: assetId } : { maskAssetId: assetId })
          }
          onRemove={(kind, assetId) => {
            deleteAssetMutation.mutate(assetId);
            if (kind === 'reference' && form.referenceAssetId === assetId) {
              updateForm({ referenceAssetId: '' });
            }
            if (kind === 'mask' && form.maskAssetId === assetId) {
              updateForm({ maskAssetId: '' });
            }
          }}
        />

        <PresetPanel
          presets={presets}
          onSave={(name) => savePresetMutation.mutate(name)}
          onLoad={(preset) =>
            updateForm({
              prompt: preset.prompt,
              negativePrompt: preset.negative_prompt,
              model: (preset.params.model || 'image-01') as MiniMaxModel,
              aspectRatio: (preset.params.aspect_ratio || '1:1') as AspectRatio,
              width: preset.params.width || 1024,
              height: preset.params.height || 1024,
              batchSize: preset.params.batch_size || 1,
              seed: preset.params.seed || 0,
              promptOptimizer: preset.params.prompt_optimizer,
              watermark: preset.params.watermark,
              styleType: (preset.params.style_type || '') as StyleType | '',
              styleWeight: preset.params.style_weight || 0.8,
              useNegativePrompt: preset.params.use_negative_prompt,
              maskMode: (preset.params.mask_mode || 'generated_in_mask') as GenerationFormState['maskMode'],
            })
          }
          onDelete={(presetId) => deletePresetMutation.mutate(presetId)}
        />
      </aside>

      <main className="app-canvas">
        {!hasApiKey && (
          <div className="notice notice-warning">
            No MiniMax API key is configured. Open Settings to add one before generating.
          </div>
        )}

        {problems.length > 0 && (
          <div className="notice notice-error">{problems.join(' ')}</div>
        )}

        <section className="panel">
          <div className="panel-header">
            <span>Preview</span>
            <span className="text-dim">
              {selectedImage ? `${selectedImage.width}×${selectedImage.height}` : ''}
            </span>
          </div>
          <div className="panel-body">
            {selectedImage ? (
              <>
                <div className="preview-stage">
                  <img src={selectedImage.url} alt={selectedImage.prompt || 'Generated image'} />
                </div>
                <div className="row">
                  <select
                    value={compareImageId}
                    onChange={(event) => setCompareImageId(event.target.value)}
                    aria-label="Compare with"
                  >
                    <option value="">Compare with…</option>
                    {images
                      .filter((image) => image.id !== selectedImage.id)
                      .map((image) => (
                        <option key={image.id} value={image.id}>
                          {image.prompt.slice(0, 40) || image.id}
                        </option>
                      ))}
                  </select>
                  <button
                    type="button"
                    className="btn"
                    onClick={() => exportMutation.mutate(selectedImage.id)}
                  >
                    Export
                  </button>
                </div>
                {compareImage && (
                  <div className="compare-grid">
                    <div className="compare-cell">
                      <span className="compare-label">Selected</span>
                      <img src={selectedImage.url} alt="Selected" />
                    </div>
                    <div className="compare-cell">
                      <span className="compare-label">Comparison</span>
                      <img src={compareImage.url} alt="Comparison" />
                    </div>
                  </div>
                )}
              </>
            ) : (
              <div className="empty-state">
                Select a generated image to preview it. Completed images appear in the gallery.
              </div>
            )}
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <span>Mask</span>
            <span className="text-dim">local composite</span>
          </div>
          <div className="panel-body">
            <div className="notice">
              MiniMax Image-01 has no alpha mask input. The mask is applied locally to the
              returned image, it is not model-side inpainting.
            </div>
            <div className="field">
              <label className="field-label" htmlFor="mask-mode">
                Composite mode
              </label>
              <select
                id="mask-mode"
                value={form.maskMode}
                onChange={(event) =>
                  updateForm({ maskMode: event.target.value as GenerationFormState['maskMode'] })
                }
              >
                <option value="generated_in_mask">Keep generated pixels inside the mask</option>
                <option value="generated_outside_mask">
                  Keep generated pixels outside the mask
                </option>
              </select>
            </div>
            <MaskCanvas
              backgroundUrl={selectedImage?.url ?? ''}
              brushSize={40}
              onMaskChange={setMaskBlob}
              onClear={() => setMaskBlob(null)}
            />
            {maskBlob && (
              <div className="notice notice-warning">
                A mask is ready. It is uploaded and applied when the next job is submitted.
              </div>
            )}
          </div>
        </section>
      </main>

      <aside className="app-gallery">
        <JobQueue
          jobs={jobs}
          onCancel={(jobId) => cancelMutation.mutate(jobId)}
          onRetry={(jobId) => retryMutation.mutate(jobId)}
          onSelect={(jobId) => {
            const job = jobs.find((item) => item.id === jobId);
            if (job && job.image_ids.length > 0) {
              setSelectedImageId(job.image_ids[0]);
            }
          }}
        />

        <Gallery
          images={images}
          selectedId={selectedImageId}
          onSelect={setSelectedImageId}
          onDelete={(imageId) => deleteImageMutation.mutate(imageId)}
          onExport={(imageId) => exportMutation.mutate(imageId)}
        />
      </aside>

      <ConsolePanel entries={consoleEntries} onClear={clearConsole} />

      <SettingsDialog
        open={settingsOpen}
        settings={settingsQuery.data ?? null}
        onClose={() => setSettingsOpen(false)}
        onSave={(apiKey, baseUrl, model) =>
          credentialsMutation.mutate({ apiKey, baseUrl, model })
        }
        onTest={() => testConnectionMutation.mutate()}
        onSaveDefaults={(defaults) => defaultsMutation.mutate(defaults)}
        onSaveUI={(ui) => {
          // Apply it straight away so the user sees the result, then
          // persist it.
          applyZoom(ui.zoom);
          uiMutation.mutate(ui);
        }}
      />

      {toast && (
        <div className={`toast toast-${toast.kind}`} role="status" aria-live="polite">
          {toast.message}
        </div>
      )}
    </div>
  );
}

export default App;
