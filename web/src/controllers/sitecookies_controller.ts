import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static override targets = ["error", "status"]

  declare readonly errorTarget: HTMLElement
  declare readonly hasErrorTarget: boolean
  declare readonly statusTarget: HTMLElement
  declare readonly hasStatusTarget: boolean

  private setError(message: string): void {
    if (this.hasErrorTarget) this.errorTarget.textContent = message
  }

  private setStatus(message: string): void {
    if (this.hasStatusTarget) this.statusTarget.textContent = message
  }

  async add(event: Event): Promise<void> {
    event.preventDefault()
    this.setError("")
    const form = event.currentTarget as HTMLFormElement
    const data = new FormData(form)
    const payload = {
      domain: String(data.get("domain") || "").trim(),
      name: String(data.get("name") || "").trim(),
      cookies: String(data.get("cookies") || "").trim(),
    }
    try {
      const response = await fetch("/api/sitecookies", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      })
      if (!response.ok) {
        this.setError(await response.text())
        return
      }
      window.location.reload()
    } catch (e) {
      this.setError(String(e))
    }
  }

  async remove(event: Event): Promise<void> {
    event.preventDefault()
    const uid = (event.currentTarget as HTMLElement).dataset.sitecookiesUidParam
    if (!uid || !window.confirm("Delete these cookies?")) return
    try {
      const response = await fetch(`/api/sitecookies/${encodeURIComponent(uid)}`, {
        method: "DELETE",
      })
      if (response.ok) window.location.reload()
    } catch (e) {
      this.setError(String(e))
    }
  }

  async saveCookieCloud(event: Event): Promise<void> {
    event.preventDefault()
    this.setStatus("")
    const form = event.currentTarget as HTMLFormElement
    const data = new FormData(form)
    const payload = {
      server: String(data.get("server") || "").trim(),
      uuid: String(data.get("uuid") || "").trim(),
      password: String(data.get("password") || ""),
      enabled: data.get("enabled") !== null,
    }
    try {
      const response = await fetch("/api/sitecookies/cookiecloud", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      })
      if (!response.ok) {
        this.setStatus(await response.text())
        return
      }
      this.setStatus("Settings saved")
    } catch (e) {
      this.setStatus(String(e))
    }
  }

  async sync(event: Event): Promise<void> {
    event.preventDefault()
    this.setStatus("Syncing...")
    try {
      const response = await fetch("/api/sitecookies/cookiecloud/sync", {
        method: "POST",
      })
      const text = await response.text()
      if (!response.ok) {
        this.setStatus(`Sync failed: ${text}`)
        return
      }
      this.setStatus("Sync complete — reloading")
      window.setTimeout(() => window.location.reload(), 1000)
    } catch (e) {
      this.setStatus(String(e))
    }
  }
}
