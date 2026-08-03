//
//  ShareViewController.swift
//  ShareExtension
//
//  Native share flow: the collection pick and the upload happen entirely inside the extension,
//  because iOS does not reliably let a share extension open its containing app (both
//  NSExtensionContext.open and the UIApplication openURL: responder hack are refused on modern
//  iOS). The user shares, picks a collection, and stays in the app they came from.
//
//  Auth: the main app mirrors its session token + appview base URL into the App Group container
//  (SharedAuthPlugin in App/AppDelegate.swift → auth.json); the extension calls the appview
//  directly with it. Images are uploaded via POST /save (the same endpoint the web uploader
//  uses); shared links are scraped via POST /api/extract-images and the picked images saved by
//  URL. No hand-off to the webview ever happens on iOS — Android keeps its send-intent flow.
//

import SwiftUI
import UIKit
import UniformTypeIdentifiers

// MARK: - What arrived from the share sheet

private enum SharePayload {
	case images([SharedImage]) // photos / image files
	case link(String) // a web page to scrape for images
}

private struct SharedImage {
	let data: Data
	let filename: String
	let mime: String
}

// MARK: - Appview client

private struct AuthConfig {
	let token: String
	let apiUrl: URL

	// Written by SharedAuthPlugin in the main app; absent means logged out.
	static func load() -> AuthConfig? {
		guard
			let dir = FileManager.default.containerURL(
				forSecurityApplicationGroupIdentifier: "group.is.currents.app"),
			let data = try? Data(contentsOf: dir.appendingPathComponent("auth.json")),
			let obj = try? JSONSerialization.jsonObject(with: data) as? [String: String],
			let token = obj["token"]
		else { return nil }
		let base = obj["apiUrl"].flatMap(URL.init(string:)) ?? URL(string: "https://api.currents.is")!
		return AuthConfig(token: token, apiUrl: base)
	}
}

private struct Collection: Identifiable {
	let uri: String
	let name: String
	let parentUri: String?
	let thumb: URL?
	var id: String { uri }
}

private struct APIError: LocalizedError {
	let message: String
	var errorDescription: String? { message }
}

private final class CurrentsAPI {
	private let config: AuthConfig
	init(_ config: AuthConfig) { self.config = config }

	private func request(_ method: String, _ path: String, query: [URLQueryItem] = []) -> URLRequest {
		var comps = URLComponents(
			url: config.apiUrl.appendingPathComponent(path), resolvingAgainstBaseURL: false)!
		if !query.isEmpty { comps.queryItems = query }
		var req = URLRequest(url: comps.url!)
		req.httpMethod = method
		req.setValue("Bearer \(config.token)", forHTTPHeaderField: "Authorization")
		return req
	}

	private func run(_ req: URLRequest) async throws -> Data {
		let (data, res) = try await URLSession.shared.data(for: req)
		let code = (res as? HTTPURLResponse)?.statusCode ?? 0
		guard (200..<300).contains(code) else {
			if code == 401 {
				throw APIError(message: "Session expired — open Currents and log in again.")
			}
			let body = (String(data: data, encoding: .utf8) ?? "")
				.trimmingCharacters(in: .whitespacesAndNewlines)
			throw APIError(message: body.isEmpty ? "Request failed (\(code))" : body)
		}
		return data
	}

	func me() async throws -> String {
		let data = try await run(request("GET", "/api/me"))
		guard let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
			let did = obj["did"] as? String
		else { throw APIError(message: "Unexpected /api/me response") }
		return did
	}

	func collections(did: String) async throws -> [Collection] {
		let data = try await run(
			request(
				"GET", "/xrpc/is.currents.feed.getActorCollections",
				query: [
					URLQueryItem(name: "actor", value: did),
					URLQueryItem(name: "limit", value: "100")
				]))
		guard let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
			let cols = obj["collections"] as? [[String: Any]]
		else { throw APIError(message: "Unexpected collections response") }
		return cols.compactMap { c in
			guard let uri = c["uri"] as? String, let name = c["name"] as? String else { return nil }
			let previews = c["previews"] as? [[String: Any]]
			let thumb = (previews?.first?["url"] as? String).flatMap(URL.init(string:))
			return Collection(uri: uri, name: name, parentUri: c["parentUri"] as? String, thumb: thumb)
		}
	}

	func extractImages(page: String) async throws -> [URL] {
		var req = request("POST", "/api/extract-images")
		req.setValue("application/json", forHTTPHeaderField: "Content-Type")
		req.httpBody = try JSONSerialization.data(withJSONObject: ["url": page])
		let data = try await run(req)
		guard let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
			let images = obj["images"] as? [String]
		else { return [] }
		return images.compactMap(URL.init(string:))
	}

	func save(image: SharedImage, collection: String) async throws {
		var form = MultipartForm()
		form.field("collection", collection)
		form.file("image", filename: image.filename, mime: image.mime, data: image.data)
		try await send(form)
	}

	func save(imageUrl: URL, page: String, collection: String) async throws {
		var form = MultipartForm()
		form.field("collection", collection)
		form.field("imageUrl", imageUrl.absoluteString)
		form.field("url", page)
		try await send(form)
	}

	private func send(_ form: MultipartForm) async throws {
		var req = request("POST", "/save")
		var form = form
		req.setValue(
			"multipart/form-data; boundary=\(form.boundary)", forHTTPHeaderField: "Content-Type")
		req.httpBody = form.finish()
		_ = try await run(req)
	}
}

private struct MultipartForm {
	let boundary = "currents-\(UUID().uuidString)"
	private var body = Data()

	mutating func field(_ name: String, _ value: String) {
		body.append(
			Data(
				"--\(boundary)\r\nContent-Disposition: form-data; name=\"\(name)\"\r\n\r\n\(value)\r\n"
					.utf8))
	}

	mutating func file(_ name: String, filename: String, mime: String, data: Data) {
		body.append(
			Data(
				"--\(boundary)\r\nContent-Disposition: form-data; name=\"\(name)\"; filename=\"\(filename)\"\r\nContent-Type: \(mime)\r\n\r\n"
					.utf8))
		body.append(data)
		body.append(Data("\r\n".utf8))
	}

	mutating func finish() -> Data {
		body.append(Data("--\(boundary)--\r\n".utf8))
		return body
	}
}

// MARK: - State

@MainActor
private final class ShareModel: ObservableObject {
	enum Phase {
		case loading
		case login
		case pickImages // link flow: choose which scraped images to keep
		case pickCollection
		case saving
		case done(String)
		case failed(String)
	}

	@Published var phase: Phase = .loading
	@Published var scraped: [URL] = []
	@Published var selected: Set<URL> = []
	@Published var collections: [Collection] = []
	var sharedImages: [SharedImage] = []

	private var pageUrl: String?
	private var api: CurrentsAPI?
	let close: () -> Void

	init(close: @escaping () -> Void) { self.close = close }

	func start(payload: SharePayload) async {
		guard let config = AuthConfig.load() else {
			phase = .login
			return
		}
		let api = CurrentsAPI(config)
		self.api = api
		do {
			let did = try await api.me()
			collections = try await api.collections(did: did)
			switch payload {
			case .images(let images):
				sharedImages = images
				phase = .pickCollection
			case .link(let page):
				pageUrl = page
				scraped = try await api.extractImages(page: page)
				if scraped.isEmpty {
					phase = .failed("No images found on that page.")
				} else {
					phase = .pickImages
				}
			}
		} catch {
			phase = .failed(error.localizedDescription)
		}
	}

	func proceedToCollections() { phase = .pickCollection }

	// nil = "Profile" (an unsorted save, empty collection URI).
	func save(to collection: Collection?) {
		guard let api else { return }
		let uri = collection?.uri ?? ""
		let name = collection?.name ?? "your profile"
		phase = .saving
		Task {
			do {
				if let pageUrl {
					for url in scraped where selected.contains(url) {
						try await api.save(imageUrl: url, page: pageUrl, collection: uri)
					}
				} else {
					for image in sharedImages {
						try await api.save(image: image, collection: uri)
					}
				}
				phase = .done(name)
				try? await Task.sleep(nanoseconds: 900_000_000)
				close()
			} catch {
				phase = .failed(error.localizedDescription)
			}
		}
	}
}

// MARK: - Views

private struct ShareRootView: View {
	@ObservedObject var model: ShareModel

	var body: some View {
		NavigationView {
			content
				.navigationTitle("Save to Currents")
				.navigationBarTitleDisplayMode(.inline)
				.toolbar {
					ToolbarItem(placement: .cancellationAction) {
						Button("Cancel") { model.close() }
					}
				}
		}
		.navigationViewStyle(.stack)
	}

	@ViewBuilder private var content: some View {
		switch model.phase {
		case .loading:
			ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
		case .login:
			message("Log in to Currents first, then share again.")
		case .failed(let text):
			message(text)
		case .pickImages:
			ImagePickGrid(model: model)
		case .pickCollection:
			CollectionList(model: model)
		case .saving:
			VStack(spacing: 12) {
				ProgressView()
				Text("Saving…").foregroundColor(.secondary)
			}
			.frame(maxWidth: .infinity, maxHeight: .infinity)
		case .done(let name):
			VStack(spacing: 12) {
				Image(systemName: "checkmark.circle.fill")
					.font(.system(size: 44))
					.foregroundColor(.green)
				Text("Saved to \(name)")
			}
			.frame(maxWidth: .infinity, maxHeight: .infinity)
		}
	}

	private func message(_ text: String) -> some View {
		Text(text)
			.multilineTextAlignment(.center)
			.foregroundColor(.secondary)
			.padding()
			.frame(maxWidth: .infinity, maxHeight: .infinity)
	}
}

private struct ImagePickGrid: View {
	@ObservedObject var model: ShareModel
	private let columns = [GridItem(.adaptive(minimum: 100), spacing: 2)]

	var body: some View {
		VStack(spacing: 0) {
			ScrollView {
				LazyVGrid(columns: columns, spacing: 2) {
					ForEach(model.scraped, id: \.self) { url in
						thumb(url)
					}
				}
			}
			Button(action: { model.proceedToCollections() }) {
				Text(model.selected.isEmpty ? "Select images to save" : "Next")
					.frame(maxWidth: .infinity)
			}
			.buttonStyle(.borderedProminent)
			.disabled(model.selected.isEmpty)
			.padding()
		}
	}

	private func thumb(_ url: URL) -> some View {
		Button {
			if model.selected.contains(url) {
				model.selected.remove(url)
			} else {
				model.selected.insert(url)
			}
		} label: {
			AsyncImage(url: url) { phase in
				if let image = phase.image {
					image.resizable().scaledToFill()
				} else {
					Color(.secondarySystemBackground)
				}
			}
			.frame(height: 110)
			.clipped()
			.overlay(alignment: .topTrailing) {
				Image(systemName: model.selected.contains(url) ? "checkmark.circle.fill" : "circle")
					.foregroundColor(.white)
					.shadow(radius: 2)
					.padding(6)
			}
		}
	}
}

private struct CollectionList: View {
	@ObservedObject var model: ShareModel

	// Roots in server order, each followed by its sections (indented).
	private var ordered: [(Collection, Bool)] {
		let roots = model.collections.filter { $0.parentUri == nil }
		return roots.flatMap { root in
			[(root, false)]
				+ model.collections.filter { $0.parentUri == root.uri }.map { ($0, true) }
		}
	}

	var body: some View {
		List {
			if !model.sharedImages.isEmpty {
				Section {
					ScrollView(.horizontal, showsIndicators: false) {
						HStack(spacing: 6) {
							ForEach(Array(model.sharedImages.enumerated()), id: \.offset) { _, img in
								if let ui = UIImage(data: img.data) {
									Image(uiImage: ui)
										.resizable()
										.scaledToFill()
										.frame(width: 56, height: 56)
										.clipShape(RoundedRectangle(cornerRadius: 8))
								}
							}
						}
					}
				}
			}
			Section {
				row(name: "Profile", thumb: nil, indented: false) { model.save(to: nil) }
			}
			Section("Collections") {
				ForEach(ordered, id: \.0.id) { pair in
					row(name: pair.0.name, thumb: pair.0.thumb, indented: pair.1) {
						model.save(to: pair.0)
					}
				}
			}
		}
	}

	private func row(
		name: String, thumb: URL?, indented: Bool, action: @escaping () -> Void
	) -> some View {
		Button(action: action) {
			HStack(spacing: 10) {
				if indented { Spacer().frame(width: 16) }
				Group {
					if let thumb {
						AsyncImage(url: thumb) { phase in
							if let image = phase.image {
								image.resizable().scaledToFill()
							} else {
								Color(.secondarySystemBackground)
							}
						}
					} else {
						Color(.secondarySystemBackground)
					}
				}
				.frame(width: 36, height: 36)
				.clipShape(RoundedRectangle(cornerRadius: 6))
				Text(name).foregroundColor(.primary)
			}
		}
	}
}

// MARK: - Entry point

class ShareViewController: UIViewController {
	private var model: ShareModel!

	override func viewDidLoad() {
		super.viewDidLoad()
		view.backgroundColor = .systemBackground

		model = ShareModel(close: { [weak self] in
			self?.extensionContext?.completeRequest(returningItems: [], completionHandler: nil)
		})
		let host = UIHostingController(rootView: ShareRootView(model: model))
		addChild(host)
		host.view.frame = view.bounds
		host.view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
		view.addSubview(host.view)
		host.didMove(toParent: self)

		Task {
			guard let payload = await loadPayload() else {
				model.phase = .failed("Nothing Currents can save was shared.")
				return
			}
			await model.start(payload: payload)
		}
	}

	private func loadPayload() async -> SharePayload? {
		guard let item = extensionContext?.inputItems.first as? NSExtensionItem,
			let attachments = item.attachments
		else { return nil }

		var images: [SharedImage] = []
		var link: String?

		for (index, attachment) in attachments.enumerated() {
			// Order matters: a Photos image also conforms to public.file-url, so check
			// image before url, otherwise it'd be mis-typed as a plain link.
			if attachment.hasItemConformingToTypeIdentifier(UTType.image.identifier) {
				if let image = try? await loadImage(attachment, index) { images.append(image) }
			} else if attachment.hasItemConformingToTypeIdentifier(UTType.url.identifier) {
				if let result = try? await attachment.loadItem(
					forTypeIdentifier: UTType.url.identifier, options: nil),
					let url = result as? URL, !url.isFileURL
				{
					link = link ?? url.absoluteString
				}
			} else if attachment.hasItemConformingToTypeIdentifier(UTType.text.identifier) {
				if let result = try? await attachment.loadItem(
					forTypeIdentifier: UTType.text.identifier, options: nil),
					let text = result as? String
				{
					link = link ?? firstHttpUrl(in: text)
				}
			}
		}

		if !images.isEmpty { return .images(images) }
		if let link { return .link(link) }
		return nil
	}

	private func loadImage(_ attachment: NSItemProvider, _ index: Int) async throws -> SharedImage? {
		let result = try await attachment.loadItem(
			forTypeIdentifier: UTType.image.identifier, options: nil)
		switch result {
		case let url as URL:
			let data = try Data(contentsOf: url)
			return prepared(data: data, filename: url.lastPathComponent, index: index)
		case let image as UIImage:
			guard let data = image.pngData() else { return nil }
			return SharedImage(data: data, filename: "shared_\(index).png", mime: "image/png")
		case let data as Data:
			return prepared(data: data, filename: "shared_\(index)", index: index)
		default:
			NSLog("ShareExtension: unexpected image payload \(type(of: result))")
			return nil
		}
	}

	// The PDS blob scope wants a concrete image mime and the pipeline expects ordinary web
	// formats, so anything else (HEIC above all) is re-encoded as JPEG.
	private func prepared(data: Data, filename: String, index: Int) -> SharedImage? {
		let webMimes = [
			"jpg": "image/jpeg", "jpeg": "image/jpeg", "png": "image/png",
			"gif": "image/gif", "webp": "image/webp"
		]
		let ext = (filename as NSString).pathExtension.lowercased()
		if let mime = webMimes[ext] {
			return SharedImage(data: data, filename: filename, mime: mime)
		}
		guard let image = UIImage(data: data), let jpeg = image.jpegData(compressionQuality: 0.9)
		else { return nil }
		return SharedImage(data: jpeg, filename: "shared_\(index).jpg", mime: "image/jpeg")
	}

	private func firstHttpUrl(in text: String) -> String? {
		guard let range = text.range(of: #"https?://\S+"#, options: .regularExpression) else {
			return nil
		}
		return String(text[range])
	}
}
