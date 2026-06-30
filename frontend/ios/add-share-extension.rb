#!/usr/bin/env ruby
# Adds the "ShareExtension" app-extension target to App.xcodeproj and wires the App Group on
# both targets. Idempotent: re-running is a no-op once the target exists. Run from frontend/ios/.
require "xcodeproj"

PROJECT_PATH = File.join(__dir__, "App", "App.xcodeproj")
EXT_NAME = "ShareExtension"
EXT_BUNDLE_ID = "is.currents.app.ShareExtension"
APP_GROUP = "group.is.currents.app"
DEPLOY_TARGET = "15.0"

project = Xcodeproj::Project.open(PROJECT_PATH)
app_target = project.targets.find { |t| t.name == "App" } or abort("App target not found")

if project.targets.any? { |t| t.name == EXT_NAME }
  puts "#{EXT_NAME} target already exists — nothing to do."
  exit 0
end

# --- Extension target ---------------------------------------------------------
ext = project.new_target(:app_extension, EXT_NAME, :ios, DEPLOY_TARGET)

# Navigator group + file references (paths are relative to SOURCE_ROOT = ios/App).
group = project.main_group.new_group(EXT_NAME, EXT_NAME)
swift_ref = group.new_reference("ShareViewController.swift")
group.new_reference("Info.plist")
group.new_reference("#{EXT_NAME}.entitlements")
ext.source_build_phase.add_file_reference(swift_ref)

ext.build_configurations.each do |config|
  s = config.build_settings
  s["PRODUCT_BUNDLE_IDENTIFIER"] = EXT_BUNDLE_ID
  s["PRODUCT_NAME"] = "$(TARGET_NAME)"
  s["INFOPLIST_FILE"] = "#{EXT_NAME}/Info.plist"
  s["CODE_SIGN_ENTITLEMENTS"] = "#{EXT_NAME}/#{EXT_NAME}.entitlements"
  s["IPHONEOS_DEPLOYMENT_TARGET"] = DEPLOY_TARGET
  s["SWIFT_VERSION"] = "5.0"
  s["GENERATE_INFOPLIST_FILE"] = "NO"
  s["MARKETING_VERSION"] = "1.0"
  s["CURRENT_PROJECT_VERSION"] = "1"
  s["TARGETED_DEVICE_FAMILY"] = "1,2"
  s["SKIP_INSTALL"] = "YES"
  s["CODE_SIGN_STYLE"] = "Automatic"
  s["ALWAYS_EMBED_SWIFT_STANDARD_LIBRARIES"] = "NO"
  s["LD_RUNPATH_SEARCH_PATHS"] = ["$(inherited)", "@executable_path/Frameworks", "@executable_path/../../Frameworks"]
end

# --- App Group entitlement on the App target ---------------------------------
app_target.build_configurations.each do |config|
  config.build_settings["CODE_SIGN_ENTITLEMENTS"] = "App/App.entitlements"
end
app_group = project.main_group.children.find { |g| g.respond_to?(:display_name) && g.display_name == "App" }
if app_group && app_group.files.none? { |f| f.display_name == "App.entitlements" }
  app_group.new_reference("App.entitlements")
end

# --- Embed the extension into the app ----------------------------------------
app_target.add_dependency(ext)
embed = app_target.new_copy_files_build_phase("Embed Foundation Extensions")
embed.symbol_dst_subfolder_spec = :plug_ins
embed.dst_path = ""
build_file = embed.add_file_reference(ext.product_reference)
build_file.settings = { "ATTRIBUTES" => ["RemoveHeadersOnCopy"] }

project.save
puts "Added #{EXT_NAME} target, App Group #{APP_GROUP}, embed phase, and dependency."
