interface Props {
  message: string;
}

export function LoadingSpinner({ message }: Props) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 text-[var(--text-muted)]">
      <svg
        className="animate-spin"
        width="28"
        height="28"
        viewBox="0 0 24 24"
        fill="none"
        aria-hidden="true"
      >
        <circle
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          strokeWidth="3"
          strokeOpacity="0.25"
        />
        <path
          d="M12 2a10 10 0 0 1 10 10"
          stroke="#1F2782"
          strokeWidth="3"
          strokeLinecap="round"
        />
      </svg>
      <p className="text-xs text-center leading-snug max-w-[160px]">{message}</p>
    </div>
  );
}
