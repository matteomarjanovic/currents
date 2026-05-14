import { Capacitor } from '@capacitor/core';

export function isNative(): boolean {
	return Capacitor.isNativePlatform();
}

export function isIos(): boolean {
	return Capacitor.getPlatform() === 'ios';
}

export function isAndroid(): boolean {
	return Capacitor.getPlatform() === 'android';
}

export function platform(): 'ios' | 'android' | 'web' {
	return Capacitor.getPlatform() as 'ios' | 'android' | 'web';
}
