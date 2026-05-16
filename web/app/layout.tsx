import type { Metadata } from "next"
import { Providers } from "@/components/providers"
import "@/app/globals.css"

export const metadata: Metadata = {
  title: "Watchlist",
  description: "Track movies and TV shows",
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body>
        <Providers>
          {children}
        </Providers>
      </body>
    </html>
  )
}