import * as i18n from '@solid-primitives/i18n'
import {
	ParentComponent,
	Suspense,
	createContext,
	createResource,
	startTransition,
	useContext, createSignal, createEffect,
} from 'solid-js'

import { dict as en_dict } from './locales/en'
import { store } from '~/store'
import { query } from '@solidjs/router'
import set = query.set

type RawDictionary = typeof en_dict;

export type Locale =
	| 'en'
	| 'ru'


type DeepPartial<T> = T extends Record<string, unknown>
	? { [K in keyof T]?: DeepPartial<T[K]> }
	: T;

const raw_dict_map: Record<Locale, () => Promise<{ dict: DeepPartial<RawDictionary> }>> = {
	en: () => null as any,
	ru: () => import('./locales/ru') as any,
}

export type Dictionary = i18n.Flatten<RawDictionary>;

const en_flat_dict: Dictionary = i18n.flatten(en_dict)

async function fetchDictionary(locale: Locale): Promise<Dictionary> {
	if (locale === 'en') return en_flat_dict

	const { dict } = await raw_dict_map[locale]()
	const flat_dict = i18n.flatten(dict) as RawDictionary
	return { ...en_flat_dict, ...flat_dict }
}

interface LocaleState {
	get locale(): Locale;

	setLocale(value: Locale): void;

	t: i18n.Translator<Dictionary>;
}

const LocaleContext = createContext<LocaleState>({} as LocaleState)

export const useTranslations = () => useContext(LocaleContext)

export const LocaleContextProvider: ParentComponent = (props) => {
	const [locale, setLocale] = createSignal<Locale>('en')

	createEffect(() => {
		if (store.user?.language_code) {
			setLocale(store.user.language_code as Locale)
		}
	})

	const [dict] = createResource(locale, fetchDictionary, { initialValue: en_flat_dict })

	const t = i18n.translator(dict, i18n.resolveTemplate)

	const state: LocaleState = {
		get locale() {
			return locale()
		},
		setLocale(value) {
			void startTransition(() => {
				set('locale', value)
			})
		},
		t,
	}

	return (
		<Suspense>
			<LocaleContext.Provider value={state}>
				{props.children}
			</LocaleContext.Provider>
		</Suspense>
	)
}
