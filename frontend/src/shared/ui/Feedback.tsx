export function Feedback(props: { tone: "error" | "success" | "info"; message: string; requestId?: string }) {
  const { tone, message, requestId } = props;

  return (
    <p role={tone === "error" ? "alert" : "status"}>
      {message}
      {requestId ? `（请求 ID：${requestId}）` : null}
    </p>
  );
}
