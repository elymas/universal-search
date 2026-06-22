export const dynamic = "force-static";

export default function NotFound() {
  return (
    <html>
      <body>
        <div
          style={{
            padding: "2rem",
            textAlign: "center",
            fontFamily: "sans-serif",
          }}
        >
          <h1>404 – Page Not Found</h1>
          <p>
            <a href="/en">Go to English documentation</a>
            {" | "}
            <a href="/ko">한국어 문서로 이동</a>
          </p>
        </div>
      </body>
    </html>
  );
}
