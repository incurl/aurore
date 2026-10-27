/**
 * The job queue.
 *
 * Preserves the InvokeAI queue experience: a live list of jobs with
 * status indicators, elapsed time and per-job actions (cancel, retry).
 */

import { useMemo } from 'react';

import type { Job } from '../../invoke-adapter/types';
import { QueueStatusDot } from '../../wrapped-invoke-ui/features/queue/ui/QueueStatusDot';
import type { QueueItemStatus } from '../../wrapped-invoke-ui/features/queue/core/types';

interface JobQueueProps {
  jobs: Job[];
  onCancel: (jobId: string) => void;
  onRetry: (jobId: string) => void;
  onSelect: (jobId: string) => void;
}

/**
 * Maps Aurore's job status onto InvokeAI's queue status.
 *
 * The vendored status dot understands the InvokeAI vocabulary, so the
 * two are bridged here rather than duplicating the component.
 */
const QUEUE_STATUS: Record<Job['status'], QueueItemStatus> = {
  pending: 'pending',
  running: 'in_progress',
  completed: 'completed',
  failed: 'failed',
  cancelled: 'canceled',
};

/**
 * Formats the elapsed time since a job was created.
 */
function elapsed(created: string): string {
  if (!created) {
    return '';
  }
  const start = new Date(created).getTime();
  if (Number.isNaN(start)) {
    return '';
  }
  const seconds = Math.max(0, Math.round((Date.now() - start) / 1000));
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${seconds % 60}s`;
}

/**
 * The job queue panel.
 */
export function JobQueue({ jobs, onCancel, onRetry, onSelect }: JobQueueProps) {
  const summary = useMemo(() => {
    const running = jobs.filter((job) => job.status === 'running').length;
    const pending = jobs.filter((job) => job.status === 'pending').length;
    return { running, pending };
  }, [jobs]);

  return (
    <section className="panel">
      <div className="panel-header">
        <span>Queue</span>
        <span className="text-dim">
          {summary.running} running · {summary.pending} pending
        </span>
      </div>
      <div className="panel-body">
        {jobs.length === 0 ? (
          <div className="empty-state">No jobs yet. Submit a prompt to start generating.</div>
        ) : (
          <div className="job-list">
            {jobs.map((job) => (
              <article key={job.id} className="job-item">
                <div className="job-item-top">
                  <QueueStatusDot status={QUEUE_STATUS[job.status]} boxSize="2" />
                  <span className="status-label">{job.status}</span>
                  <span className="grow" />
                  <span className="text-dim mono">{elapsed(job.created)}</span>
                </div>

                <button
                  type="button"
                  className="job-prompt"
                  title={job.prompt}
                  onClick={() => onSelect(job.id)}
                  style={{ background: 'none', border: 'none', padding: 0, textAlign: 'left' }}
                >
                  {job.prompt || '(no prompt)'}
                </button>

                {job.status === 'failed' && job.error_message && (
                  <div className="job-error">{job.error_message}</div>
                )}

                <div className="row row-end">
                  {(job.status === 'pending' || job.status === 'running') && (
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={() => onCancel(job.id)}
                    >
                      Cancel
                    </button>
                  )}
                  {(job.status === 'failed' || job.status === 'cancelled') && (
                    <button type="button" className="btn btn-sm" onClick={() => onRetry(job.id)}>
                      Retry
                    </button>
                  )}
                  {job.image_ids.length > 0 && (
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={() => onSelect(job.id)}
                    >
                      {job.image_ids.length} image{job.image_ids.length === 1 ? '' : 's'}
                    </button>
                  )}
                </div>
              </article>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
