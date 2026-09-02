export function ConnectorIcon({
  src,
  className = "h-8 w-8 object-contain",
  boxClassName = "flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-white ring-1 ring-line",
}: {
  src?: string;
  className?: string;
  boxClassName?: string;
}) {
  if (!src) return null;
  return (
    <span className={boxClassName}>
      {/* Brand marks from /public/connectors — white plate, no stretch */}
      <img src={src} alt="" className={className} />
    </span>
  );
}
