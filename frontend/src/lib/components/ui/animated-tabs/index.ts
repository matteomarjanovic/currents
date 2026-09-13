import Content, { type TabsContentProps } from './animated-tabs-content.svelte';
import List, {
	tabsListVariants,
	type TabsListProps,
	type TabsListVariant
} from './animated-tabs-list.svelte';
import Trigger, { type TabsTriggerProps } from './animated-tabs-trigger.svelte';
import Root, { type TabsProps } from './animated-tabs.svelte';

export {
	Content,
	List,
	Root,
	//
	Root as AnimatedTabs,
	Content as TabsContent,
	List as TabsList,
	tabsListVariants,
	Trigger as TabsTrigger,
	Trigger,
	type TabsContentProps,
	type TabsListProps,
	type TabsListVariant,
	type TabsProps,
	type TabsTriggerProps
};
