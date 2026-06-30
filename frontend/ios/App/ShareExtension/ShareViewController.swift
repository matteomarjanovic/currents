//
//  ShareViewController.swift
//  ShareExtension
//
//  Receives images / links from the iOS share sheet and hands them to the main app via the
//  `currents://` URL scheme (parsed in AppDelegate into the send-intent plugin's ShareStore).
//  Shared files are copied into the App Group container so the main app can read them.
//
//  NB: query values are passed RAW — URLComponents percent-encodes them when building the URL
//  and the AppDelegate decodes them once when parsing, so the JS receives clean values (matching
//  Android). Do not add manual percent-encoding here or you'll double-encode.
//

import Social
import UIKit
import UniformTypeIdentifiers

private final class ShareItem {
	var title: String?
	var type: String?
	var url: String?
}

class ShareViewController: UIViewController {
	private let appGroupId = "group.is.currents.app"
	private let appScheme = "currents"
	private var shareItems: [ShareItem] = []

	override func viewDidAppear(_ animated: Bool) {
		super.viewDidAppear(animated)
		extensionContext?.completeRequest(returningItems: [], completionHandler: nil)
	}

	override func viewDidLoad() {
		super.viewDidLoad()
		shareItems.removeAll()

		guard let item = extensionContext?.inputItems.first as? NSExtensionItem,
			let attachments = item.attachments
		else {
			openHostApp()
			return
		}

		Task {
			do {
				try await withThrowingTaskGroup(of: ShareItem.self) { group in
					for (index, attachment) in attachments.enumerated() {
						// Order matters: a Photos image also conforms to public.file-url, so check
						// image/movie before url, otherwise it'd be mis-typed as a plain link.
						if attachment.hasItemConformingToTypeIdentifier(UTType.image.identifier) {
							group.addTask { try await self.handleImage(attachment, index) }
						} else if attachment.hasItemConformingToTypeIdentifier(UTType.movie.identifier) {
							group.addTask { try await self.handleMovie(attachment) }
						} else if attachment.hasItemConformingToTypeIdentifier(UTType.url.identifier) {
							group.addTask { try await self.handleUrl(attachment) }
						} else if attachment.hasItemConformingToTypeIdentifier(UTType.text.identifier) {
							group.addTask { try await self.handleText(attachment) }
						}
					}
					for try await shared in group { self.shareItems.append(shared) }
				}
			} catch {
				NSLog("ShareExtension: failed to load shared item: \(error.localizedDescription)")
			}
			self.openHostApp()
		}
	}

	// MARK: - Type handlers

	private func handleUrl(_ attachment: NSItemProvider) async throws -> ShareItem {
		let result = try await attachment.loadItem(forTypeIdentifier: UTType.url.identifier, options: nil)
		let shared = ShareItem()
		if let url = result as? URL {
			if url.isFileURL {
				shared.title = url.lastPathComponent
				shared.type = "application/" + url.pathExtension.lowercased()
				shared.url = copyToContainer(url)
			} else {
				shared.title = url.absoluteString
				shared.type = "text/plain"
				shared.url = url.absoluteString
			}
		}
		return shared
	}

	private func handleText(_ attachment: NSItemProvider) async throws -> ShareItem {
		let result = try await attachment.loadItem(forTypeIdentifier: UTType.text.identifier, options: nil)
		let shared = ShareItem()
		shared.title = result as? String
		shared.type = "text/plain"
		return shared
	}

	private func handleMovie(_ attachment: NSItemProvider) async throws -> ShareItem {
		let result = try await attachment.loadItem(forTypeIdentifier: UTType.movie.identifier, options: nil)
		let shared = ShareItem()
		if let url = result as? URL {
			shared.title = url.lastPathComponent
			shared.type = "video/" + url.pathExtension.lowercased()
			shared.url = copyToContainer(url)
		}
		return shared
	}

	private func handleImage(_ attachment: NSItemProvider, _ index: Int) async throws -> ShareItem {
		let result = try await attachment.loadItem(forTypeIdentifier: UTType.image.identifier, options: nil)
		let shared = ShareItem()
		switch result {
		case let url as URL:
			shared.title = url.lastPathComponent
			shared.type = "image/" + url.pathExtension.lowercased()
			shared.url = copyToContainer(url)
		case let image as UIImage:
			shared.title = "shared_\(index).png"
			shared.type = "image/png"
			shared.url = writePng(image, index)
		default:
			NSLog("ShareExtension: unexpected image payload \(type(of: result))")
		}
		return shared
	}

	// MARK: - App Group container

	private func containerURL() -> URL? {
		FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: appGroupId)
	}

	private func copyToContainer(_ src: URL) -> String? {
		guard let dir = containerURL() else { return nil }
		let dest = dir.appendingPathComponent(src.lastPathComponent)
		try? FileManager.default.removeItem(at: dest)
		do {
			try Data(contentsOf: src).write(to: dest)
			return dest.absoluteString
		} catch {
			NSLog("ShareExtension: copy failed: \(error.localizedDescription)")
			return nil
		}
	}

	private func writePng(_ image: UIImage, _ index: Int) -> String? {
		guard let dir = containerURL(), let data = image.pngData() else { return nil }
		let dest = dir.appendingPathComponent("shared_\(index).png")
		do {
			try data.write(to: dest)
			return dest.absoluteString
		} catch {
			NSLog("ShareExtension: png write failed: \(error.localizedDescription)")
			return nil
		}
	}

	// MARK: - Hand off to the host app

	private func openHostApp() {
		var comps = URLComponents()
		comps.scheme = appScheme
		comps.host = "shared"
		comps.queryItems = shareItems.flatMap { item in
			[
				URLQueryItem(name: "title", value: item.title ?? ""),
				URLQueryItem(name: "description", value: ""),
				URLQueryItem(name: "type", value: item.type ?? ""),
				URLQueryItem(name: "url", value: item.url ?? "")
			]
		}
		guard let url = comps.url else { return }
		openURL(url)
	}

	// Walk the responder chain to reach UIApplication from inside the extension. UIApplication.open
	// is unavailable to app extensions at compile time, so dispatch the (still-live) openURL:
	// selector at runtime instead.
	@objc private func openURL(_ url: URL) {
		let selector = NSSelectorFromString("openURL:")
		var responder: UIResponder? = self
		while let current = responder {
			if let app = current as? UIApplication, app.responds(to: selector) {
				app.perform(selector, with: url)
				return
			}
			responder = current.next
		}
	}
}
